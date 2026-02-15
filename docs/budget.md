# Budget Subsystem

## What it does
Tracks spend against provider and global limits, and blocks requests when limits are reached.

## Structure
- `internal/budget/pricing.go`: embedded model pricing map.
- `internal/budget/engine.go`: budget set/clear/status/reset/enforcement logic.
- `internal/daemon/server.go`: pre-request budget check and post-request spend accumulation.

## Key decisions
- Budgets are persisted in SQLite (`budgets` table).
- Reset windows are UTC-based (`daily`, `monthly`).
- Budget checks run before upstream forwarding; spend is added after successful completion.

## Limitations
- Unknown models estimate to zero cost.
- Requests that push spend over limit are blocked on the next request cycle.
