# GPU Booking Agent (Google ADK)

A Google ADK-based multi-agent system for managing GPU resource bookings, exposed via the A2A protocol for Kagenti deployment.

## Architecture

```
                       ┌────────────────────────────────────────┐
                       │ gpu_booking_assistant (root_agent)     │
                       │                                        │
                       │  ┌──────────────────────────────────┐  │
                       │  │  availability_agent (read-only)  │  │
                       │  │  Tools: get_config,              │  │
                       │  │         list_bookings,           │  │
                       │  │         check_availability       │  │
                       │  └──────────────────────────────────┘  │
                       │                                        │
                       │  ┌──────────────────────────────────┐  │
                       │  │  reservation_agent (mutating)    │  │
                       │  │  Tools: create_booking,          │  │
                       │  │         bulk_book,               │  │
                       │  │         cancel_booking,          │  │
                       │  │         check_availability       │  │
                       │  └──────────────────────────────────┘  │
                       └────────────────┬───────────────────────┘
                                        │ MCP (streamable-http)
                                        ▼
                       ┌────────────────────────────────────────┐
                       │ GPU Booking MCP Tool Server            │
                       │ (FastMCP, wraps Go backend API)        │
                       └────────────────┬───────────────────────┘
                                        │ HTTP
                                        ▼
                       ┌────────────────────────────────────────┐
                       │ Go Booking Backend (:8080)             │
                       │ SQLite + Kueue sync                    │
                       └────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Python 3.11+
- `uv` package manager
- Go backend + MCP tool server running (see `mcp/gpu_booking_tool/`)
- A Google Gemini API key (or Ollama for local LLMs)

### Running Locally

```bash
# 1. Start the Go backend (terminal 1)
cd server
ADMIN_PASSWORD=changeme DEV_USER=testuser KUEUE_SYNC_ENABLED=false go run .

# 2. Start the MCP tool server (terminal 2)
cd mcp/gpu_booking_tool
uv sync && uv run gpu_booking_tool.py

# 3. Start the ADK agent (terminal 3)
cd a2a/gpu_booking_agent
cp .env.gemini .env
# Edit .env to set your GOOGLE_API_KEY
uv sync && uv run adk web .
```

The ADK web UI opens at `http://localhost:8000`. Select `gpu_booking_assistant` and start chatting.

### Running as A2A Server

```bash
# Start as a standalone A2A server (for Kagenti or other A2A clients)
uv run server
```

The A2A server starts on `http://0.0.0.0:8001`.
Agent card available at `http://localhost:8001/.well-known/agent.json`.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GOOGLE_API_KEY` | (required for Gemini) | Gemini API key |
| `ADK_MODEL` | `gemini-2.5-flash` | LLM model identifier |
| `MCP_URL` | `http://localhost:8000/mcp` | MCP tool server URL |
| `HOST` | `0.0.0.0` | A2A server bind host |
| `PORT` | `8001` | A2A server bind port |
| `LOG_LEVEL` | `INFO` | Python logging level |
| `OPENAI_API_BASE` | _(unset)_ | Base URL for OpenAI-compatible LLMs (Llama Stack, Ollama) |
| `OPENAI_API_KEY` | _(unset)_ | API key for OpenAI-compatible endpoint |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _(unset)_ | OTLP gRPC endpoint for tracing (disabled if unset) |
| `OTEL_SERVICE_NAME` | `gpu-booking-agent` | Service name for OTEL traces |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Use plaintext gRPC to OTEL collector |

## Sub-Agents

| Agent | Purpose | Tools |
|-------|---------|-------|
| `availability_agent` | Read-only GPU queries | get_config, list_bookings, check_availability |
| `reservation_agent` | Mutating booking operations | create_booking, bulk_book, cancel_booking, check_availability |

The root agent delegates to sub-agents based on user intent. Read-only queries go to the availability agent; booking mutations go to the reservation agent.

## Example Interactions

```
User: What GPUs are available tomorrow?
Agent: On 2026-04-18, here's the availability:
       - H200 Full GPU: 6 of 8 free (2 reserved)
       - MIG 3g.71gb: 8 of 8 free
       - MIG 2g.35gb: 7 of 8 free (1 consumed, overridable)
       - MIG 1g.18gb: 14 of 16 free

User: Book me 2 MIG 3g.71gb GPUs for Monday to Wednesday next week
Agent: I'll book 2 MIG 3g.71gb units from 2026-04-20 to 2026-04-22.
       Shall I proceed? (full day, 00:00-24:00 UTC)

User: Yes, go ahead
Agent: Done! Booked 2x MIG 3g.71gb for:
       - 2026-04-20 (slots 0, 1)
       - 2026-04-21 (slots 0, 1)
       - 2026-04-22 (slots 0, 1)
       Total: 6 bookings, 3.0 GPU equivalents

User: Cancel all my bookings for next Wednesday
Agent: I found 2 bookings for 2026-04-22:
       - MIG 3g.71gb slot 0 (id: abc123)
       - MIG 3g.71gb slot 1 (id: def456)
       Shall I cancel both?
```

## License

Apache License 2.0
