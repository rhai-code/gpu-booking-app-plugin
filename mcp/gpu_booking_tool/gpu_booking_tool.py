"""
GPU Booking MCP Tool Server

FastMCP server that wraps the Go booking backend API, exposing GPU resource
booking operations as MCP tools for AI agents.
"""

import json
import logging
import os

from fastmcp import FastMCP
from pydantic import ValidationError

from starlette.responses import JSONResponse

from observability import setup_otel
from providers.http_provider import HTTPProvider
from schemas import BookingConfig, BookingsListResponse

logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO"),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(__name__)

DEFAULT_USER = os.getenv("DEFAULT_USER", "agent-user")

provider: HTTPProvider | None = None
mcp = FastMCP("GPU Booking Tool")


def _init_provider():
    """Initialize the HTTP provider and OTEL. Called once at startup."""
    global provider
    if provider is not None:
        return

    setup_otel()

    booking_api_url = os.getenv("BOOKING_API_URL", "http://localhost:8080")
    logger.info("Connecting to Go backend at %s", booking_api_url)
    provider = HTTPProvider(booking_api_url)


@mcp.tool()
async def get_config() -> str:
    """Get the GPU resource configuration for this cluster.

    Returns the available GPU resource types (H200 full GPUs and MIG partitions),
    their counts, share ratios, GPU equivalents, the booking window in days,
    and total CPU/memory for the node pool.

    Use this tool first to understand what resources are available before
    checking availability or making bookings.
    """
    result = await provider.get_config()
    if "error" in result:
        return json.dumps(result, indent=2)
    try:
        validated = BookingConfig.model_validate(result)
        return validated.model_dump_json(by_alias=True, indent=2)
    except ValidationError as exc:
        logger.warning("Config response failed validation: %s", exc)
        return json.dumps(result, indent=2)


@mcp.tool()
async def list_bookings(user: str = DEFAULT_USER) -> str:
    """List all GPU bookings across all resources and dates.

    Returns every booking in the system, including:
    - Reserved bookings (user-created reservations that have priority)
    - Consumed bookings (auto-synced from Kueue LocalQueue GPU usage)
    - Active Kubernetes reservations per user
    - The current authenticated user

    Args:
        user: The username to authenticate as (maps to X-Forwarded-User header).
    """
    result = await provider.list_bookings(user)
    if "error" in result:
        return json.dumps(result, indent=2)
    try:
        validated = BookingsListResponse.model_validate(result)
        return validated.model_dump_json(by_alias=True, indent=2)
    except ValidationError as exc:
        logger.warning("Bookings response failed validation: %s", exc)
        return json.dumps(result, indent=2)


@mcp.tool()
async def check_availability(
    resource_type: str,
    date: str,
    user: str = DEFAULT_USER,
) -> str:
    """Check how many GPU slots are available for a specific resource type on a date.

    Computes availability by comparing current bookings against the total slot
    count from the configuration. Returns free, reserved, and consumed counts.

    Reserved slots cannot be overridden. Consumed slots can be overridden by
    making a reservation (the consumed booking is automatically evicted).

    Args:
        resource_type: The GPU resource type to check, e.g. "nvidia.com/gpu",
            "nvidia.com/mig-3g.71gb", "nvidia.com/mig-2g.35gb", "nvidia.com/mig-1g.18gb".
        date: The date to check availability for, in YYYY-MM-DD format (UTC).
        user: The username to authenticate as.
    """
    config = await provider.get_config()
    if "error" in config:
        return json.dumps({"error": "Failed to fetch config", "detail": config})

    bookings_data = await provider.list_bookings(user)
    if "error" in bookings_data:
        return json.dumps({"error": "Failed to fetch bookings", "detail": bookings_data})

    total_slots = 0
    resource_name = resource_type
    for res in config.get("resources", []):
        if res.get("type") == resource_type:
            total_slots = res.get("count", 0)
            resource_name = res.get("name", resource_type)
            break

    if total_slots == 0:
        return json.dumps({"error": f"Unknown resource type: {resource_type}"})

    reserved_slots: list[int] = []
    consumed_slots: list[int] = []
    for b in bookings_data.get("bookings", []):
        try:
            if b.get("resource") == resource_type and b.get("date") == date:
                slot = b.get("slotIndex", -1)
                if b.get("source") == "reserved":
                    reserved_slots.append(slot)
                else:
                    consumed_slots.append(slot)
        except (TypeError, AttributeError):
            logger.warning("Skipping malformed booking entry: %s", b)
            continue

    free_count = max(0, total_slots - len(reserved_slots) - len(consumed_slots))
    free_indices = [
        i for i in range(total_slots) if i not in reserved_slots and i not in consumed_slots
    ]
    overridable_indices = consumed_slots

    return json.dumps({
        "resource_type": resource_type,
        "resource_name": resource_name,
        "date": date,
        "total_slots": total_slots,
        "free_count": free_count,
        "reserved_count": len(reserved_slots),
        "consumed_count": len(consumed_slots),
        "free_slot_indices": free_indices,
        "overridable_slot_indices": overridable_indices,
        "summary": (
            f"{resource_name} on {date}: {free_count} free, "
            f"{len(reserved_slots)} reserved, {len(consumed_slots)} consumed "
            f"(overridable) out of {total_slots} total"
        ),
    }, indent=2)


@mcp.tool()
async def create_booking(
    resource: str,
    slot_index: int,
    date: str,
    user: str = DEFAULT_USER,
    description: str = "",
    start_hour: int = 0,
    end_hour: int = 24,
) -> str:
    """Reserve a single GPU slot for a specific date.

    Creates a "reserved" booking that takes priority over consumed (Kueue-synced)
    bookings. If the slot has a consumed booking, it will be automatically evicted.
    If the slot already has a reserved booking, the request returns an error.

    Use check_availability first to find free slot indices.

    Args:
        resource: GPU resource type, e.g. "nvidia.com/gpu" or "nvidia.com/mig-3g.71gb".
        slot_index: The unit index to book (0-based, from check_availability free_slot_indices).
        date: Booking date in YYYY-MM-DD format (UTC).
        user: The username to authenticate as.
        description: Optional booking description (max 160 characters).
        start_hour: Start hour in UTC (0-23). Default 0 for full day.
        end_hour: End hour in UTC (1-24). Default 24 for full day.
    """
    result = await provider.create_booking(
        user=user,
        resource=resource,
        slot_index=slot_index,
        date=date,
        description=description,
        start_hour=start_hour,
        end_hour=end_hour,
    )
    return json.dumps(result, indent=2)


@mcp.tool()
async def bulk_book(
    resources: dict[str, int],
    start_date: str,
    end_date: str,
    user: str = DEFAULT_USER,
    description: str = "",
    start_hour: int = 0,
    end_hour: int = 24,
) -> str:
    """Book multiple GPU resources across a date range in one operation.

    Automatically finds available slot indices for each resource type.
    Consumed bookings are evicted to make room; reserved bookings are skipped.
    Partial success is possible -- some resources may be booked while others fail.

    Args:
        resources: Map of resource type to count, e.g.
            {"nvidia.com/gpu": 2, "nvidia.com/mig-3g.71gb": 1}.
        start_date: First booking date in YYYY-MM-DD format (UTC).
        end_date: Last booking date in YYYY-MM-DD format (UTC), inclusive.
        user: The username to authenticate as.
        description: Optional booking description (max 160 characters).
        start_hour: Start hour in UTC (0-23). Default 0 for full day.
        end_hour: End hour in UTC (1-24). Default 24 for full day.
    """
    result = await provider.bulk_book(
        user=user,
        resources=resources,
        start_date=start_date,
        end_date=end_date,
        description=description,
        start_hour=start_hour,
        end_hour=end_hour,
    )
    return json.dumps(result, indent=2)


@mcp.tool()
async def cancel_booking(
    booking_id: str,
    user: str = DEFAULT_USER,
) -> str:
    """Cancel a GPU booking by its ID.

    Only the booking owner can cancel their own reserved bookings.
    Consumed bookings (auto-synced from Kueue) cannot be cancelled by normal
    users -- they are removed automatically when the workload stops.

    Args:
        booking_id: The unique ID of the booking to cancel.
        user: The username to authenticate as (must match booking owner).
    """
    result = await provider.cancel_booking(user=user, booking_id=booking_id)
    return json.dumps(result, indent=2)


def run_server():
    """Entry point for running the MCP server."""
    _init_provider()

    transport = os.getenv("MCP_TRANSPORT", "streamable-http")
    host = os.getenv("HOST", "0.0.0.0")
    port = int(os.getenv("PORT", "8000"))

    @mcp.custom_route("/healthz", methods=["GET"])
    async def healthz(request):
        return JSONResponse({"status": "ok"})

    mcp.run(transport=transport, host=host, port=port)


if __name__ == "__main__":
    run_server()
