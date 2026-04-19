"""HTTP provider that calls the Go booking backend API with retry logic."""

import json
import logging
import os
from typing import Any

import httpx

logger = logging.getLogger(__name__)

MAX_RETRIES = 3
SA_TOKEN_PATH = "/var/run/secrets/kubernetes.io/serviceaccount/token"


def _safe_json(resp: httpx.Response) -> dict[str, Any]:
    """Parse response body as JSON, handling plain-text and malformed bodies."""
    ct = resp.headers.get("content-type", "")
    if "application/json" in ct:
        try:
            return resp.json()
        except (json.JSONDecodeError, ValueError):
            return {"error": resp.text[:200].strip() or f"HTTP {resp.status_code}"}
    return {"error": resp.text.strip() or f"HTTP {resp.status_code}"}


def _read_sa_token() -> str | None:
    """Read the Kubernetes ServiceAccount token for in-cluster auth."""
    try:
        with open(SA_TOKEN_PATH) as f:
            return f.read().strip()
    except FileNotFoundError:
        return None


class HTTPProvider:
    """Calls the Go booking backend at BOOKING_API_URL with retries.

    Auth modes (checked in order):
      1. BOOKING_API_TOKEN env var -> Bearer token (explicit)
      2. ServiceAccount token at SA_TOKEN_PATH -> Bearer token (in-cluster)
      3. X-Forwarded-User header (standalone gpu-booking-app fallback)
    """

    def __init__(self, base_url: str, timeout: float = 30.0):
        self.base_url = base_url.rstrip("/")
        self._bearer_token = os.getenv("BOOKING_API_TOKEN") or _read_sa_token()
        if self._bearer_token:
            logger.info("Using Bearer token auth for backend API")
        else:
            logger.info("Using X-Forwarded-User header auth for backend API")
        transport = httpx.AsyncHTTPTransport(retries=MAX_RETRIES)
        self.client = httpx.AsyncClient(
            base_url=self.base_url,
            timeout=timeout,
            transport=transport,
        )

    async def close(self):
        """Close the underlying HTTP client. Call on shutdown."""
        await self.client.aclose()

    def _headers(self, user: str | None = None) -> dict[str, str]:
        headers: dict[str, str] = {
            "Content-Type": "application/json",
            "X-Internal-Request": "true",
        }
        if self._bearer_token:
            headers["Authorization"] = f"Bearer {self._bearer_token}"
        if user:
            headers["X-Forwarded-User"] = user
        return headers

    async def _request(
        self,
        method: str,
        path: str,
        *,
        user: str | None = None,
        json: dict | None = None,
        params: dict | None = None,
        expected_errors: tuple[int, ...] = (),
    ) -> dict[str, Any]:
        """Unified request method with structured error handling."""
        headers = self._headers(user)
        try:
            resp = await self.client.request(
                method, path, headers=headers, json=json, params=params
            )
        except httpx.ConnectError as exc:
            logger.error("Connection to backend failed: %s", exc)
            return {"error": "backend_unreachable", "detail": str(exc)}
        except httpx.TimeoutException as exc:
            logger.error("Backend request timed out: %s", exc)
            return {"error": "backend_timeout", "detail": str(exc)}
        except httpx.HTTPError as exc:
            logger.error("HTTP transport error on %s %s: %s", method, path, exc)
            return {"error": "transport_error", "detail": str(exc)}

        if resp.status_code in expected_errors:
            body = _safe_json(resp)
            code = {409: "conflict", 403: "forbidden", 404: "not_found"}.get(
                resp.status_code, f"http_{resp.status_code}"
            )
            return {"error": code, "detail": body if isinstance(body, dict) else body}

        if resp.status_code >= 400:
            body = _safe_json(resp)
            logger.warning(
                "Backend returned %d for %s %s: %s",
                resp.status_code, method, path, body,
            )
            return {"error": f"http_{resp.status_code}", "detail": body}

        return _safe_json(resp)

    async def get_config(self) -> dict[str, Any]:
        return await self._request("GET", "/api/config")

    async def list_bookings(self, user: str) -> dict[str, Any]:
        return await self._request("GET", "/api/bookings", user=user)

    async def create_booking(
        self,
        user: str,
        resource: str,
        slot_index: int,
        date: str,
        description: str = "",
        start_hour: int = 0,
        end_hour: int = 24,
    ) -> dict[str, Any]:
        payload = {
            "resource": resource,
            "slotIndex": slot_index,
            "date": date,
            "slotType": "full",
            "description": description,
            "startHour": start_hour,
            "endHour": end_hour,
        }
        return await self._request(
            "POST", "/api/bookings",
            user=user, json=payload, expected_errors=(409,),
        )

    async def bulk_book(
        self,
        user: str,
        resources: dict[str, int],
        start_date: str,
        end_date: str,
        description: str = "",
        start_hour: int = 0,
        end_hour: int = 24,
    ) -> dict[str, Any]:
        payload = {
            "resources": resources,
            "startDate": start_date,
            "endDate": end_date,
            "description": description,
            "startHour": start_hour,
            "endHour": end_hour,
        }
        return await self._request(
            "POST", "/api/bookings/bulk",
            user=user, json=payload, expected_errors=(409,),
        )

    async def cancel_booking(self, user: str, booking_id: str) -> dict[str, Any]:
        return await self._request(
            "DELETE", "/api/bookings",
            user=user, params={"id": booking_id}, expected_errors=(403, 404),
        )
