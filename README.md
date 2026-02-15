# Tokfence

One place for all your AI keys with full control over every token your agents burn.

Tokfence is a local-first daemon and CLI that sits between AI tools and upstream model APIs. It injects credentials, logs request metadata, estimates cost, enforces budgets, supports provider kill-switches, and exposes shell-ready base URL exports.

## Features

- Local proxy on `127.0.0.1:9471` with provider path routing.
- Vault-backed auth injection (macOS Keychain or encrypted file backend).
- SQLite request logs with model/tokens/latency/status/caller metadata.
- Cost estimation via embedded pricing map.
- Budget enforcement (`daily`/`monthly`) with HTTP 429 responses.
- Provider revocation and global `kill` / `unkill`.
- Per-provider RPM rate limiting.
- Native-looking macOS menu UI via SwiftBar widget integration.

## Quickstart

```bash
# Build
go build -o tokfence ./cmd/tokfence

# Add keys (macOS keychain backend by default)
./tokfence vault add anthropic sk-ant-...
./tokfence vault add openai sk-...

# Start daemon
./tokfence start -d

# Configure shell
# Add this to your shell profile:
eval "$(./tokfence env)"

# Verify
./tokfence status
./tokfence log -f
```

For encrypted-file vault usage, set:

```bash
export TOKFENCE_VAULT_PASSPHRASE='your-strong-passphrase'
```

## Commands

```bash
# Daemon
tokfence start [-d]
tokfence stop
tokfence status

# Vault
tokfence vault add <provider> <key|->
tokfence vault remove <provider>
tokfence vault list
tokfence vault rotate <provider> <new-key|->
tokfence vault export
tokfence vault import <file>

# Observability
tokfence log [request-id]
tokfence log -f --provider openai --since 1h
tokfence stats --period 7d --by model

# Budget
tokfence budget set <provider|global> <amount_usd> <daily|monthly>
tokfence budget status
tokfence budget clear <provider|global>

# Control
tokfence revoke <provider>
tokfence restore <provider>
tokfence kill
tokfence unkill
tokfence ratelimit set <provider> <rpm>
tokfence ratelimit clear <provider>

# Shell integration
tokfence env [--shell bash|zsh|fish] [--provider <provider>]

# Desktop widget (SwiftBar)
tokfence widget install [--refresh 20]
tokfence widget render
tokfence widget uninstall
```

## macOS Desktop UI (SwiftBar)

Install [SwiftBar](https://swiftbar.app/), then install the Tokfence widget:

```bash
tokfence widget install --refresh 20
```

The widget shows:
- daemon status (online/offline)
- today's requests, token totals, and estimated cost
- budget usage progress bars
- revoked providers and vault coverage
- one-click actions (`start`, `stop`, `kill`, `unkill`, logs, stats)

If your binary is not in `PATH`, pass it explicitly:

```bash
tokfence widget install --binary /absolute/path/to/tokfence
```

## Config

Default config path: `~/.tokfence/config.toml`

```toml
[daemon]
port = 9471
host = "127.0.0.1"

[logging]
db_path = "~/.tokfence/tokfence.db"
retention_days = 90

[notifications]
budget_warning_percent = 80
```

Provider defaults are embedded and can be overridden in `[providers.<name>]` blocks.

## Development

```bash
go test ./...
make build
make test
```

No external API calls are used in tests.
