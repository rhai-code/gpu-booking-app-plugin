FROM python:3.13-slim AS builder

COPY --from=ghcr.io/astral-sh/uv:0.11 /uv /usr/local/bin/uv

ENV UV_COMPILE_BYTECODE=1
ENV UV_LINK_MODE=copy

WORKDIR /app

COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-install-project --no-dev

COPY . .
RUN uv sync --frozen --no-dev


FROM python:3.13-slim

RUN groupadd -g 0 appuser || true

COPY --from=builder /app /app
RUN chgrp -R 0 /app && chmod -R g=u /app
WORKDIR /app

ENV PATH="/app/.venv/bin:$PATH"
ENV PORT=8000
ENV HOST=0.0.0.0
ENV MCP_TRANSPORT=streamable-http

EXPOSE 8000

USER 1001

CMD ["python", "gpu_booking_tool.py"]
