# Proxy Subsystem

## What it does
Tokfence runs a local HTTP gateway on `127.0.0.1:9471` and forwards requests to provider upstreams based on the first URL path segment.

## Structure
- `internal/proxy/router.go`: provider/path parsing and upstream URL resolution.
- `internal/proxy/streaming.go`: stream detection and SSE chunk copy helpers.
- `internal/proxy/headers.go`: auth header stripping and provider auth injection.
- `internal/daemon/server.go`: runtime proxy pipeline and forwarding implementation.

## Key decisions
- Path contract is `/{provider}/{upstream-path...}`.
- Existing auth headers are stripped before forwarding.
- Streaming requests (`stream=true`) are flushed chunk-by-chunk to preserve interactive latency.
- Provider upstream base paths (for example OpenRouter `/api`) are preserved.

## Limitations
- SSE parsing is metadata-oriented and tolerant; malformed events are skipped.
- No WebSocket support by design.
