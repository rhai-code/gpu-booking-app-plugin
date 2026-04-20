"""HTTP provider that calls the Go booking backend API with retry logic.

Supports two auth modes:
  1. User token forwarding: when a real OpenShift OAuth token is available
     (threaded from Console -> Agent -> MCP), it is sent as a Bearer token
     and the Go backend validates via TokenReview.
  2. Fallback: X-Internal-Request + X-Forwarded-User for health/config
     endpoints where no user token is available (e.g. startup probes).
"""

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

    Auth is determined per-request:
      - If a user_token is provided (real OAuth token forwarded from the
        Console proxy via the ADK agent), it is sent as Authorization: Bearer.
        The Go backend validates it via TokenReview.
      - Otherwise, falls back to X-Internal-Request: true (restricted to
        unauthenticated endpoints like /api/config by the backend).
    """

    def __init__(self, base_url: str, timeout: float = 30.0):
        self.base_url = base_url.rstrip("/")
        self._sa_token = os.getenv("BOOKING_API_TOKEN") or _read_sa_token()
        if self._sa_token:
            logger.info("ServiceAccount/static token available for fallback auth")
        else:
            logger.info("No SA token; user token forwarding is the only auth path")
        transport = httpx.AsyncHTTPTransport(retries=MAX_RETRIES)
        self.client = httpx.AsyncClient(
            base_url=self.base_url,
            timeout=timeout,
            transport=transport,
        )

    async def close(self):
        """Close the underlying HTTP client. Call on shutdown."""
        await self.client.aclose()

    def _headers(
        self, user: str | None = None, user_token: str | None = None
    ) -> dict[str, str]:
        headers: dict[str, str] = {"Content-Type": "application/json"}
        if user_token:
            headers["Authorization"] = f"Bearer {user_token}"
        else:
            headers["X-Internal-Request"] = "true"
            if self._sa_token:
                headers["Authorization"] = f"Bearer {self._sa_token}"
        if user:
            headers["X-Forwarded-User"] = user
        return headers

    async def _request(
        self,
        method: str,
        path: str,
        *,
        user: str | None = None,
        user_token: str | None = None,
        json: dict | None = None,
        params: dict | None = None,
        expected_errors: tuple[int, ...] = (),
    ) -> dict[str, Any]:
        """Unified request method with structured error handling."""
        headers = self._headers(user, user_token=user_token)
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

    async def get_config(self, user_token: str | None = None) -> dict[str, Any]:
        return await self._request("GET", "/api/config", user_token=user_token)

    async def list_bookings(
        self, user: str, user_token: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            "GET", "/api/bookings", user=user, user_token=user_token
        )

    async def create_booking(
        self,
        user: str,
        resource: str,
        slot_index: int,
        date: str,
        description: str = "",
        start_hour: int = 0,
        end_hour: int = 24,
        user_token: str | None = None,
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
            user=user, user_token=user_token, json=payload,
            expected_errors=(409,),
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
        user_token: str | None = None,
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
            user=user, user_token=user_token, json=payload,
            expected_errors=(409,),
        )

    async def cancel_booking(
        self, user: str, booking_id: str, user_token: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            "DELETE", "/api/bookings",
            user=user, user_token=user_token,
            params={"id": booking_id}, expected_errors=(403, 404),
        )
