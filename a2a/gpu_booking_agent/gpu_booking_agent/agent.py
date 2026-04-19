"""
GPU Booking Agent -- Google ADK with MCP Tools

A multi-agent system for managing GPU resource bookings on an OpenShift cluster.
Uses ADK's MCPToolset to connect to the FastMCP tool server, which wraps the
Go booking backend API.

Supports dynamic model/endpoint configuration via ADK's before_model_callback:
when an `endpointId` is passed in the A2A message metadata, the callback reads
it from RunConfig.custom_metadata, fetches the endpoint config from the Go
backend, and overrides llm_request.model so LiteLLM routes to the correct
provider.

Architecture:
  root_agent (BookingAssistant)
    ├── availability_agent (read-only queries)
    └── reservation_agent (mutating operations)
"""

import logging
import os
from typing import Optional

from google.adk.agents import LlmAgent
from google.adk.agents.callback_context import CallbackContext
from google.adk.models.lite_llm import LiteLlm
from google.adk.models.llm_request import LlmRequest
from google.adk.models.llm_response import LlmResponse
from google.adk.tools.mcp_tool import McpToolset
from google.adk.tools.mcp_tool.mcp_session_manager import StreamableHTTPConnectionParams

from gpu_booking_agent.endpoint_config import DEFAULT_MODEL, get_model_for_request
from gpu_booking_agent.observability import setup_otel

logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO"),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(__name__)

MCP_URL = os.getenv("MCP_URL", "http://localhost:8000/mcp")


def _dynamic_model_callback(
    callback_context: CallbackContext, llm_request: LlmRequest
) -> Optional[LlmResponse]:
    """Switch LLM model per-request based on A2A metadata endpointId.

    ADK's A2A layer maps message metadata into RunConfig.custom_metadata
    under the key 'a2a_metadata'. This callback reads endpointId from there,
    fetches the endpoint config from the Go backend, and overrides
    llm_request.model so LiteLLM routes to the correct provider.
    """
    metadata = None
    try:
        run_config = callback_context.run_config
        if run_config and run_config.custom_metadata:
            metadata = run_config.custom_metadata.get("a2a_metadata")
    except (AttributeError, KeyError):
        pass

    model = get_model_for_request(metadata)
    if model != DEFAULT_MODEL:
        llm_request.model = model
        logger.info("Dynamic model switch: using %s for this LLM call", model)

    return None


SYSTEM_INSTRUCTION = """\
You are a GPU Booking Assistant that helps users manage GPU resource reservations
on an OpenShift cluster with NVIDIA H200 GPUs and MIG (Multi-Instance GPU) partitions.

You are a coordinator agent. You do NOT have direct access to GPU tools.
Instead, you delegate tasks to your specialized sub-agents:

- **availability_agent**: Handles ALL read-only queries -- checking configuration,
  listing bookings, computing availability. Use transfer_to_agent to send queries
  about what resources exist, what's available, or current bookings.
- **reservation_agent**: Handles ALL booking mutations -- creating, bulk-booking,
  and cancelling reservations. Use transfer_to_agent for any booking or cancellation request.

## IMPORTANT
- NEVER try to call get_config, list_bookings, check_availability, create_booking,
  bulk_book, or cancel_booking yourself. You do NOT have these tools.
- ALWAYS use transfer_to_agent to delegate to the appropriate sub-agent.
- For queries about resources or availability -> transfer to availability_agent.
- For booking or cancellation requests -> transfer to reservation_agent.

## Booking Rules
- Dates are in UTC (YYYY-MM-DD format).
- Hours are in UTC (0-23 start, 1-24 end). Default is full day (0-24).
- "Reserved" bookings are user-created and have priority.
- "Consumed" bookings are auto-synced from Kueue workloads and can be overridden
  by making a reservation (the consumed booking is automatically evicted).
- Only reserved-vs-reserved conflicts are blocked (slot_taken error).
- Descriptions are limited to 160 characters.

## Communication Style
- Be concise but informative.
- Show availability as a clear summary (e.g., "3 of 8 H200 GPUs are free on 2026-04-18").
- When booking succeeds, confirm the details (resource, date, hours).
- If a booking fails, explain why and suggest alternatives.
"""

AVAILABILITY_INSTRUCTION = """\
You handle read-only GPU resource queries. Your job is to check what resources
are available and report status to the user.

Use these tools:
- get_config: to understand what GPU resources exist and their counts
- list_bookings: to see all current bookings
- check_availability: to find free slots for a specific resource and date

Always provide clear summaries like:
- "On 2026-04-18, there are 5 of 8 H200 GPUs free, 2 reserved, 1 consumed (overridable)"
- "MIG 1g.18gb has 12 of 16 units available tomorrow"

When asked about availability across multiple days, check each day and summarize.
"""

RESERVATION_INSTRUCTION = """\
You handle GPU booking mutations: creating and cancelling reservations.

Use these tools:
- create_booking: reserve a single GPU slot (use after checking availability)
- bulk_book: book multiple resources across a date range
- cancel_booking: cancel a booking by its ID

IMPORTANT RULES:
- Always confirm with the user before creating or cancelling bookings.
- For single-slot bookings, use create_booking with a specific slot_index from
  check_availability results.
- For multi-resource or multi-day requests, use bulk_book which auto-finds slots.
- Show the user what was booked after success (resource type, count, dates, hours).
- If a slot is taken, suggest checking availability for alternatives.
"""


def _make_litellm_model(model_name: str) -> LiteLlm:
    """Create a LiteLlm connector for the given model name.

    Using LiteLlm (not a bare string) as the agent's model= ensures
    llm_request.model overrides in before_model_callback are routed
    through LiteLLM to the correct provider (Gemini, OpenAI, Ollama, etc.).
    """
    litellm_model = model_name
    if not any(model_name.startswith(p) for p in ("gemini/", "openai/", "ollama/", "groq/", "anthropic/")):
        litellm_model = f"gemini/{model_name}"
    return LiteLlm(model=litellm_model)


def build_agents(model_name: str):
    """Build agent hierarchy using LiteLLM as the model connector."""
    model = _make_litellm_model(model_name)

    availability_toolset = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=MCP_URL),
        tool_filter=["get_config", "list_bookings", "check_availability"],
    )

    reservation_toolset = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=MCP_URL),
        tool_filter=["create_booking", "bulk_book", "cancel_booking", "check_availability"],
    )

    availability_agent = LlmAgent(
        model=model,
        name="availability_agent",
        description=(
            "Handles read-only GPU resource queries: checking configuration, "
            "listing bookings, and computing availability for specific resources and dates."
        ),
        instruction=AVAILABILITY_INSTRUCTION,
        tools=[availability_toolset],
        before_model_callback=_dynamic_model_callback,
    )

    reservation_agent = LlmAgent(
        model=model,
        name="reservation_agent",
        description=(
            "Handles GPU booking mutations: creating single or bulk reservations "
            "and cancelling existing bookings. Confirms actions with the user before executing."
        ),
        instruction=RESERVATION_INSTRUCTION,
        tools=[reservation_toolset],
        before_model_callback=_dynamic_model_callback,
    )

    root = LlmAgent(
        model=model,
        name="gpu_booking_assistant",
        description=(
            "A GPU booking assistant that helps users manage NVIDIA H200 GPU "
            "and MIG partition reservations on an OpenShift cluster."
        ),
        instruction=SYSTEM_INSTRUCTION,
        sub_agents=[availability_agent, reservation_agent],
        before_model_callback=_dynamic_model_callback,
    )

    return root


root_agent = build_agents(DEFAULT_MODEL)


def main():
    """Entry point: expose the agent as an A2A server via uvicorn."""
    import uvicorn
    from google.adk.a2a.utils.agent_to_a2a import to_a2a

    setup_otel()

    host = os.getenv("HOST", "0.0.0.0")
    port = int(os.getenv("PORT", "8001"))

    logger.info(
        "Starting GPU Booking Agent on %s:%d, MCP_URL=%s, DEFAULT_MODEL=%s",
        host, port, MCP_URL, DEFAULT_MODEL,
    )

    a2a_app = to_a2a(root_agent, port=port)
    uvicorn.run(a2a_app, host=host, port=port)


if __name__ == "__main__":
    main()
