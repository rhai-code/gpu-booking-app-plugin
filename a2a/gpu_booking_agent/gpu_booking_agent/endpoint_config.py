"""
Dynamic LLM endpoint configuration.

When the A2A message metadata contains an `endpointId`, this module fetches
the endpoint config from the Go backend and returns the appropriate LiteLLM
model string (e.g. "gemini/gemini-2.5-flash", "openai/meta-llama/...").

For provider-specific auth, it also sets the required environment variables
so LiteLLM can pick them up automatically.
"""

import logging
import os
from typing import Optional

import httpx

logger = logging.getLogger(__name__)

BACKEND_URL = os.getenv("BACKEND_URL", "http://gpu-booking-server:9080")
_RAW_DEFAULT = os.getenv("ADK_MODEL", "gemini-2.5-flash")


def _to_litellm_format(model: str) -> str:
    """Ensure a model string is in LiteLLM provider/model format."""
    if any(model.startswith(p) for p in ("gemini/", "openai/", "ollama/", "groq/", "anthropic/")):
        return model
    return f"gemini/{model}"


DEFAULT_MODEL = _to_litellm_format(_RAW_DEFAULT)


def _exchange_maas_token(registry_url: str, bearer_token: str) -> str:
    """Exchange an OpenShift bearer token for a MaaS session token.

    Mirrors the token exchange in rhai-openshift-skills-plugin
    pkg/maas/client.go Authenticate().
    """
    token_url = registry_url.rstrip("/") + "/v1/tokens"
    resp = httpx.post(
        token_url,
        json={"expiration": "720h"},
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {bearer_token}",
        },
        timeout=15.0,
        verify=False,
    )
    resp.raise_for_status()
    token = resp.json().get("token", "")
    if not token:
        raise ValueError("empty token in MaaS token exchange response")
    return token


def _build_litellm_model(endpoint: dict) -> str:
    """Convert an endpoint config dict into a LiteLLM model string.

    LiteLLM uses the pattern "provider/model-name" for routing.
    See: https://docs.litellm.ai/docs/providers

    Also sets the required environment variables for auth / base URL.
    Note: setting os.environ is not thread-safe for concurrent requests
    with different endpoints. Acceptable for single-replica deployments.
    """
    provider = endpoint.get("provider_type", "openai-compatible")
    model = endpoint.get("model_name", "")
    url = endpoint.get("url", "")
    api_key = endpoint.get("api_key", "")

    if provider == "gemini":
        if api_key:
            os.environ["GOOGLE_API_KEY"] = api_key
        return f"gemini/{model}" if not model.startswith("gemini/") else model

    if provider == "ollama":
        if url:
            os.environ["OLLAMA_API_BASE"] = url
        return f"ollama/{model}" if not model.startswith("ollama/") else model

    if provider == "rhoai-maas":
        registry_url = url.rstrip("/")
        try:
            session_token = _exchange_maas_token(registry_url, api_key)
        except Exception:
            logger.exception("MaaS token exchange failed for %s", registry_url)
            raise

        # Inference URL: strip /maas-api suffix, then /llm/{model_slug}/v1
        base = registry_url
        for suffix in ("/maas-api", "/maas-api/"):
            if base.endswith(suffix):
                base = base[: -len(suffix)]
                break
        inference_url = f"{base}/llm/{model}/v1"

        os.environ["OPENAI_API_BASE"] = inference_url
        os.environ["OPENAI_API_KEY"] = session_token
        logger.info("MaaS: inference at %s", inference_url)
        return f"openai/{model}" if not model.startswith("openai/") else model

    # openai-compatible (Llama Stack, vLLM, etc.)
    if url:
        os.environ["OPENAI_API_BASE"] = url
    if api_key:
        os.environ["OPENAI_API_KEY"] = api_key
    return f"openai/{model}" if not model.startswith("openai/") else model


def fetch_endpoint_config(
    endpoint_id: int, user_token: str | None = None
) -> Optional[dict]:
    """Fetch endpoint configuration from the Go backend.

    When a user_token is provided (real OpenShift OAuth token), it is sent
    as a Bearer token and the backend validates via TokenReview.  Falls back
    to X-Internal-Request for the /api/llm-endpoints path which is on the
    backend's internal bypass allowlist.
    """
    headers: dict[str, str] = {}
    if user_token:
        headers["Authorization"] = f"Bearer {user_token}"
    else:
        headers["X-Internal-Request"] = "true"

    try:
        resp = httpx.get(
            f"{BACKEND_URL}/api/llm-endpoints/get",
            params={"id": str(endpoint_id)},
            headers=headers,
            timeout=10.0,
        )
        if resp.status_code != 200:
            logger.warning(
                "Failed to fetch endpoint %d: HTTP %d", endpoint_id, resp.status_code
            )
            return None
        return resp.json()
    except Exception:
        logger.exception("Error fetching endpoint config for id=%d", endpoint_id)
        return None


def get_model_for_request(
    metadata: Optional[dict] = None, user_token: str | None = None
) -> str:
    """Resolve the LiteLLM model string for a request.

    If metadata contains an endpointId, fetch the config and build the
    model string. Otherwise, return the default model.

    Returns a LiteLLM-format string (e.g. "gemini/gemini-2.5-flash").
    """
    if not metadata:
        return DEFAULT_MODEL

    endpoint_id = metadata.get("endpointId")
    if not endpoint_id:
        return DEFAULT_MODEL

    try:
        eid = int(endpoint_id)
    except (ValueError, TypeError):
        logger.warning("Invalid endpointId in metadata: %s", endpoint_id)
        return DEFAULT_MODEL

    config = fetch_endpoint_config(eid, user_token=user_token)
    if not config:
        logger.warning("Falling back to default model for endpoint %d", eid)
        return DEFAULT_MODEL

    model = _build_litellm_model(config)
    logger.info("Resolved endpoint %d to model: %s", eid, model)
    return model
