"""OpenTelemetry setup for the GPU Booking MCP Tool."""

import logging
import os

logger = logging.getLogger(__name__)


def setup_otel() -> None:
    """Configure OpenTelemetry tracing with OTLP exporter.

    Reads configuration from standard OTEL environment variables:
      OTEL_EXPORTER_OTLP_ENDPOINT   -- collector endpoint (empty = disabled)
      OTEL_SERVICE_NAME              -- service name for traces
      OTEL_EXPORTER_OTLP_INSECURE   -- "true" for plaintext gRPC (default in-cluster)
    """
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
    if not endpoint:
        logger.info("OTEL tracing disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
        return

    service_name = os.getenv("OTEL_SERVICE_NAME", "gpu-booking-tool")
    insecure = os.getenv("OTEL_EXPORTER_OTLP_INSECURE", "true").lower() == "true"

    try:
        from opentelemetry import trace
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanExporter
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
            OTLPSpanExporter,
        )
        from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

        resource = Resource.create({"service.name": service_name})
        provider = TracerProvider(resource=resource)
        exporter = OTLPSpanExporter(endpoint=endpoint, insecure=insecure)
        provider.add_span_processor(BatchSpanExporter(exporter))
        trace.set_tracer_provider(provider)

        HTTPXClientInstrumentor().instrument()

        logger.info("OTEL tracing enabled, exporting to %s (insecure=%s)", endpoint, insecure)
    except ImportError:
        logger.warning(
            "OpenTelemetry packages not installed; tracing disabled. "
            "Install opentelemetry-sdk, opentelemetry-exporter-otlp-proto-grpc, "
            "and opentelemetry-instrumentation-httpx to enable."
        )
    except Exception:
        logger.exception("Failed to initialize OTEL tracing")


