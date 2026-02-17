# Configuration Subsystem

## What it does
Loads daemon, logging, notification, and provider upstream settings from `~/.tokfence/config.toml` with secure defaults.

## Structure
- `internal/config/config.go`: defaults, TOML loading, path expansion, and merge logic.

## Key decisions
- Precedence: defaults < config file < CLI/runtime overrides.
- Daemon defaults to localhost only (`127.0.0.1`).
- Data directory creation enforces secure permissions.
- Optional Unix socket transport is enabled via `[daemon.socket_path]` with secure parent permissions and short socket path validation.
- ImmuneFence can be enabled with `[daemon].immune_enabled = true`.

### Relevant daemon config keys

- `daemon.socket_path`: path to Unix socket (`~/.tokfence/tokfence.sock` by default).
- `daemon.immune_enabled`: enable request gating and sensor enforcement.
- `risk_defaults.initial_state`: initial risk state (`GREEN`/`YELLOW`/`ORANGE`/`RED`).

## Limitations
- Unsupported/unknown config keys are ignored by TOML decode behavior.
