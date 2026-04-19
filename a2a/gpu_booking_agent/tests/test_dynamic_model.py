"""Tests for dynamic model switching via before_model_callback."""

import os
from unittest.mock import MagicMock, patch

import pytest


class TestBuildLitellmModel:
    """Tests for endpoint_config._build_litellm_model."""

    def test_gemini_provider(self):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        endpoint = {
            "provider_type": "gemini",
            "model_name": "gemini-2.5-flash",
            "api_key": "test-key",
        }
        result = _build_litellm_model(endpoint)
        assert result == "gemini/gemini-2.5-flash"
        assert os.environ.get("GOOGLE_API_KEY") == "test-key"

    def test_gemini_already_prefixed(self):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        endpoint = {
            "provider_type": "gemini",
            "model_name": "gemini/gemini-2.0-pro",
            "api_key": "",
        }
        result = _build_litellm_model(endpoint)
        assert result == "gemini/gemini-2.0-pro"

    def test_openai_compatible_provider(self):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        endpoint = {
            "provider_type": "openai-compatible",
            "model_name": "meta-llama/Llama-3.1-8B",
            "url": "https://maas.example.com/v1",
            "api_key": "sk-test",
        }
        result = _build_litellm_model(endpoint)
        assert result == "openai/meta-llama/Llama-3.1-8B"
        assert os.environ.get("OPENAI_API_BASE") == "https://maas.example.com/v1"
        assert os.environ.get("OPENAI_API_KEY") == "sk-test"

    def test_ollama_provider(self):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        endpoint = {
            "provider_type": "ollama",
            "model_name": "llama3:latest",
            "url": "http://ollama.local:11434",
            "api_key": "",
        }
        result = _build_litellm_model(endpoint)
        assert result == "ollama/llama3:latest"
        assert os.environ.get("OLLAMA_API_BASE") == "http://ollama.local:11434"

    def test_default_provider(self):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        endpoint = {
            "model_name": "gpt-4o",
            "url": "https://api.openai.com/v1",
            "api_key": "sk-real",
        }
        result = _build_litellm_model(endpoint)
        assert result == "openai/gpt-4o"

    @patch("gpu_booking_agent.endpoint_config._exchange_maas_token")
    def test_rhoai_maas_provider(self, mock_exchange):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        mock_exchange.return_value = "session-token-xyz"
        endpoint = {
            "provider_type": "rhoai-maas",
            "model_name": "ibm-granite-2b-gpu",
            "url": "https://rh-ai.apps.example.com/maas-api",
            "api_key": "ocp-bearer-token",
        }
        result = _build_litellm_model(endpoint)
        assert result == "openai/ibm-granite-2b-gpu"
        assert os.environ.get("OPENAI_API_BASE") == "https://rh-ai.apps.example.com/llm/ibm-granite-2b-gpu/v1"
        assert os.environ.get("OPENAI_API_KEY") == "session-token-xyz"
        mock_exchange.assert_called_once_with(
            "https://rh-ai.apps.example.com/maas-api", "ocp-bearer-token"
        )

    @patch("gpu_booking_agent.endpoint_config._exchange_maas_token")
    def test_rhoai_maas_without_maas_api_suffix(self, mock_exchange):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        mock_exchange.return_value = "tok-123"
        endpoint = {
            "provider_type": "rhoai-maas",
            "model_name": "llama-32-3b",
            "url": "https://gateway.example.com",
            "api_key": "bearer",
        }
        result = _build_litellm_model(endpoint)
        assert result == "openai/llama-32-3b"
        assert os.environ.get("OPENAI_API_BASE") == "https://gateway.example.com/llm/llama-32-3b/v1"

    @patch("gpu_booking_agent.endpoint_config._exchange_maas_token")
    def test_rhoai_maas_token_exchange_failure(self, mock_exchange):
        from gpu_booking_agent.endpoint_config import _build_litellm_model

        mock_exchange.side_effect = Exception("connection refused")
        endpoint = {
            "provider_type": "rhoai-maas",
            "model_name": "model-x",
            "url": "https://bad.example.com/maas-api",
            "api_key": "tok",
        }
        with pytest.raises(Exception, match="connection refused"):
            _build_litellm_model(endpoint)


class TestGetModelForRequest:
    """Tests for endpoint_config.get_model_for_request."""

    def test_no_metadata_returns_default(self):
        from gpu_booking_agent.endpoint_config import DEFAULT_MODEL, get_model_for_request

        assert get_model_for_request(None) == DEFAULT_MODEL
        assert get_model_for_request({}) == DEFAULT_MODEL

    def test_no_endpoint_id_returns_default(self):
        from gpu_booking_agent.endpoint_config import DEFAULT_MODEL, get_model_for_request

        assert get_model_for_request({"other": "data"}) == DEFAULT_MODEL

    def test_invalid_endpoint_id_returns_default(self):
        from gpu_booking_agent.endpoint_config import DEFAULT_MODEL, get_model_for_request

        assert get_model_for_request({"endpointId": "abc"}) == DEFAULT_MODEL

    @patch("gpu_booking_agent.endpoint_config.fetch_endpoint_config")
    def test_valid_endpoint_id_fetches_and_builds(self, mock_fetch):
        from gpu_booking_agent.endpoint_config import get_model_for_request

        mock_fetch.return_value = {
            "provider_type": "openai-compatible",
            "model_name": "meta-llama/Llama-3.1-8B",
            "url": "https://maas.example.com/v1",
            "api_key": "sk-test",
        }
        result = get_model_for_request({"endpointId": 42})
        assert result == "openai/meta-llama/Llama-3.1-8B"
        mock_fetch.assert_called_once_with(42)

    @patch("gpu_booking_agent.endpoint_config.fetch_endpoint_config")
    def test_fetch_failure_returns_default(self, mock_fetch):
        from gpu_booking_agent.endpoint_config import DEFAULT_MODEL, get_model_for_request

        mock_fetch.return_value = None
        result = get_model_for_request({"endpointId": 99})
        assert result == DEFAULT_MODEL


class TestDynamicModelCallback:
    """Tests for agent._dynamic_model_callback."""

    def _make_callback_context(self, metadata=None):
        """Build a mock CallbackContext with run_config.custom_metadata."""
        ctx = MagicMock()
        if metadata is not None:
            run_config = MagicMock()
            run_config.custom_metadata = {"a2a_metadata": metadata}
            ctx.run_config = run_config
        else:
            ctx.run_config = None
        return ctx

    def _make_llm_request(self, model=None):
        """Build a mock LlmRequest."""
        req = MagicMock()
        req.model = model
        return req

    @patch("gpu_booking_agent.agent.get_model_for_request")
    def test_no_metadata_does_not_override(self, mock_get_model):
        from gpu_booking_agent.agent import DEFAULT_MODEL, _dynamic_model_callback

        mock_get_model.return_value = DEFAULT_MODEL
        ctx = self._make_callback_context(metadata=None)
        req = self._make_llm_request()

        result = _dynamic_model_callback(callback_context=ctx, llm_request=req)

        assert result is None
        assert req.model != "openai/some-model"

    @patch("gpu_booking_agent.agent.get_model_for_request")
    def test_with_endpoint_id_overrides_model(self, mock_get_model):
        from gpu_booking_agent.agent import _dynamic_model_callback

        mock_get_model.return_value = "openai/meta-llama/Llama-3.1-8B"
        ctx = self._make_callback_context(metadata={"endpointId": 42})
        req = self._make_llm_request()

        result = _dynamic_model_callback(callback_context=ctx, llm_request=req)

        assert result is None
        assert req.model == "openai/meta-llama/Llama-3.1-8B"
        mock_get_model.assert_called_once_with({"endpointId": 42})

    @patch("gpu_booking_agent.agent.get_model_for_request")
    def test_default_model_not_overridden(self, mock_get_model):
        from gpu_booking_agent.agent import DEFAULT_MODEL, _dynamic_model_callback

        mock_get_model.return_value = DEFAULT_MODEL

        ctx = self._make_callback_context(metadata={"endpointId": 1})
        req = self._make_llm_request(model="original")

        result = _dynamic_model_callback(callback_context=ctx, llm_request=req)

        assert result is None
        assert req.model == "original"

    def test_broken_context_does_not_crash(self):
        from gpu_booking_agent.agent import _dynamic_model_callback

        ctx = MagicMock()
        ctx.run_config = None
        req = self._make_llm_request(model="original")

        result = _dynamic_model_callback(callback_context=ctx, llm_request=req)
        assert result is None
        assert req.model == "original"


class TestMakeLitellmModel:
    """Tests for agent._make_litellm_model."""

    def test_bare_gemini_string(self):
        from gpu_booking_agent.agent import _make_litellm_model

        model = _make_litellm_model("gemini-2.5-flash")
        assert model.model == "gemini/gemini-2.5-flash"

    def test_already_prefixed(self):
        from gpu_booking_agent.agent import _make_litellm_model

        model = _make_litellm_model("openai/gpt-4o")
        assert model.model == "openai/gpt-4o"

    def test_ollama_prefixed(self):
        from gpu_booking_agent.agent import _make_litellm_model

        model = _make_litellm_model("ollama/llama3")
        assert model.model == "ollama/llama3"
