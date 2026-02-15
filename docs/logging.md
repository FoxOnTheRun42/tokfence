# Logging Subsystem

## What it does
Records request metadata for every proxied call into SQLite, including provider/model/tokens/cost/latency/caller identity.

## Structure
- `internal/logger/logger.go`: schema, migrations, insert/query APIs.
- `internal/logger/parser.go`: model, token usage, SSE usage, request hash extraction.
- `internal/process/identify.go`: best-effort caller detection via headers and local process lookup.

## Key decisions
- Logs contain metadata only; request and response bodies are not persisted.
- Request IDs use ULID for sortable identifiers.
- Streaming requests are logged after stream completion with accumulated usage.

## Limitations
- Process identification is best-effort and may return empty on unsupported environments.
- Cost estimation depends on known model pricing map entries.
