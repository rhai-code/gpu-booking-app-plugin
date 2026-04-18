# GPU Booking MCP Tool

An MCP tool server that wraps the GPU booking app's Go backend API, exposing GPU resource booking operations as MCP tools for AI agents.

## Tools

| Tool | Description |
|------|-------------|
| `get_config` | Get GPU resource types, counts, and booking window |
| `list_bookings` | List all bookings across all resources and dates |
| `check_availability` | Check free/reserved/consumed slots for a resource on a date |
| `create_booking` | Reserve a single GPU slot |
| `bulk_book` | Book multiple resources across a date range |
| `cancel_booking` | Cancel a booking by ID |

## Quick Start

### Prerequisites

- Python 3.11+
- `uv` package manager
- Go backend running (see `server/` in the repo root)

### Running Locally

```bash
# Start the Go backend first (in another terminal)
cd server
ADMIN_PASSWORD=changeme DEV_USER=testuser KUEUE_SYNC_ENABLED=false go run .

# Run the MCP tool server
cd mcp/gpu_booking_tool
uv sync
uv run gpu_booking_tool.py
```

The MCP server starts on `http://0.0.0.0:8000`.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BOOKING_API_URL` | `http://localhost:8080` | Go backend base URL |
| `DEFAULT_USER` | `agent-user` | Default user for bookings |
| `MCP_TRANSPORT` | `streamable-http` | MCP transport protocol |
| `HOST` | `0.0.0.0` | Server bind host |
| `PORT` | `8000` | Server bind port |
| `LOG_LEVEL` | `INFO` | Logging level |

## GPU Resource Types

| Resource | Type | Count | GPU Equivalent |
|----------|------|-------|---------------|
| H200 Full GPU | `nvidia.com/gpu` | 8 | 1.0 |
| MIG 3g.71gb | `nvidia.com/mig-3g.71gb` | 8 | 0.5 |
| MIG 2g.35gb | `nvidia.com/mig-2g.35gb` | 8 | 0.25 |
| MIG 1g.18gb | `nvidia.com/mig-1g.18gb` | 16 | 0.125 |

## Architecture

```
Agent (ADK/LangGraph/etc.)
    │
    ▼ MCP streamable-http
gpu_booking_tool.py (FastMCP)
    │
    ▼ HTTP
Go Backend (:8080)
    │
    ▼
SQLite DB + Kueue API
```

## License

Apache License 2.0
