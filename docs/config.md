# Configuration Subsystem

## What it does
Loads daemon, logging, notification, and provider upstream settings from `~/.tokfence/config.toml` with secure defaults.

## Structure
- `internal/config/config.go`: defaults, TOML loading, path expansion, and merge logic.

## Key decisions
- Precedence: defaults < config file < CLI/runtime overrides.
- Daemon defaults to localhost only (`127.0.0.1`).
- Data directory creation enforces secure permissions.

## Limitations
- Unsupported/unknown config keys are ignored by TOML decode behavior.
