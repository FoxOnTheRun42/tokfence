# Watch (Key Leak Detector)

## Purpose

`tokfence watch` detects potential API key misuse by reconciling:

- local Tokfence proxy logs (requests/tokens/cost)
- provider-side usage API totals for the same period

If remote usage is meaningfully higher than local usage, Tokfence emits an alert.  
In continuous mode, it also detects idle-time drift: no local traffic for a configured window while remote usage still increases.

Important: provider usage APIs can have reporting delay. Alerts should be treated as suspicion signals that require confirmation.

## Command

```bash
tokfence watch [--provider <name>] [--period 24h] [--interval 15m]
```

Key flags:

- `--once`: run one poll and exit.
- `--json`: machine-readable report.
- `--threshold-usd`: cost delta threshold (default `1.0`).
- `--threshold-tokens`: token delta threshold (default `1000`).
- `--threshold-requests`: request count delta threshold (default `1`).
- `--idle-window`: idle drift detection window (default `30m`).
- `--auto-revoke`: automatically revoke provider on alert (use with care; reporting lag can create false positives).
- `--usage-endpoint provider=url`: override provider usage endpoint.

## Data Flow

1. Load config and vault keys.
2. Resolve watched providers (explicit `--provider` or vault-backed providers).
3. Read local totals from SQLite logs (`stats` + latest request timestamp).
4. Query provider usage endpoint(s).
5. Parse usage payload into normalized totals.
6. Compare remote vs local and evaluate thresholds.
7. Emit JSON/text report; optionally auto-revoke provider.

## Design Decisions

- Local-only operation: no external telemetry service.
- Default thresholds reduce false positives from small billing jitter.
- Endpoint override supports provider API changes without code changes.
- Alerts are deterministic and include raw fetch error context for debugging.

## Limits

- Provider usage APIs are not standardized; some providers may require endpoint overrides.
- Provider usage APIs can be delayed/inconsistent; temporary deltas are possible without key compromise.
- Cost comparisons can be skewed if provider pricing differs from Tokfence estimation for unknown models.
- Idle drift requires continuous mode to compare current poll vs prior poll.
