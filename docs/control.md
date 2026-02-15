# Control Subsystem

## What it does
Provides operator controls to revoke providers, emergency-stop all providers, and enforce per-provider request rate limits.

## Structure
- `internal/logger/logger.go`: provider status and rate limit persistence tables.
- `internal/daemon/middleware.go`: token-bucket enforcement and JSON error responses.
- `cmd/tokfence/main.go`: `revoke`, `restore`, `kill`, `unkill`, `ratelimit` command wiring.

## Key decisions
- Revocation state is checked before every upstream call.
- `kill` and `unkill` update all configured providers atomically.
- Rate limiting uses in-memory token buckets with persisted RPM settings.

## Limitations
- Rate limiter state is process-local and resets on daemon restart.
