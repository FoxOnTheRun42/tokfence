# Tokfence

## The immune system for AI agent infrastructure.

AI agents need API keys. Every framework stores them in plaintext config files. CVE-2026-25253 proved what happens: 17,500 OpenClaw instances had their keys exfiltrated through a 1-click RCE.

Tokfence makes this impossible. Keys never leave the vault. Agents get a local proxy URL. Done.

---

## The Problem

Every AI agent framework — OpenClaw, LangChain, CrewAI, AutoGPT — requires API keys in config files, environment variables, or runtime memory. This creates three attack surfaces:

**1. Config exfiltration.** Agent configs with plaintext keys get committed to repos, leaked through RCE, or read by malicious extensions.

**2. Uncontrolled spend.** An agent with a raw API key has unlimited access. One runaway loop can burn through thousands in minutes.

**3. Zero visibility.** When keys are scattered across tools, there's no single place to see what's being called, how much it costs, or whether something is wrong.

---

## What Tokfence Does

Tokfence is a local daemon that sits between your AI agents and provider APIs.

```
Agent → http://localhost:9471/openai/v1/chat/completions
                    ↓
             [ Tokfence Daemon ]
             - injects real key from encrypted vault
             - logs request (model, tokens, cost, latency)
             - enforces budget cap
             - checks risk state
                    ↓
         https://api.openai.com/v1/chat/completions
              (with real Authorization header)
```

The agent never sees the real key. It gets a proxy URL. Tokfence handles authentication, logging, budgeting, and security enforcement.

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Tokfence Daemon                │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Vault    │  │  Proxy   │  │  ImmuneFence  │  │
│  │ Keychain  │  │  Router  │  │  Capability   │  │
│  │ Argon2    │  │  Stream  │  │  Risk State   │  │
│  │ Ed25519   │  │  Headers │  │  Sensors      │  │
│  └──────────┘  └──────────┘  │  Canary        │  │
│                               └───────────────┘  │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Budget   │  │  Logger  │  │  Transport    │  │
│  │  Engine   │  │  SQLite  │  │  UDS + TCP    │  │
│  │  Rate     │  │  Stats   │  │  0660 perms   │  │
│  │  Limits   │  │  Parser  │  │  Cleanup      │  │
│  └──────────┘  └──────────┘  └───────────────┘  │
│                                                  │
│  ┌──────────────────────────────────────────┐    │
│  │           OpenClaw Launcher              │    │
│  │  Docker orchestration, config gen,       │    │
│  │  health checks, guided setup             │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  ┌──────────────────────────────────────────┐    │
│  │        macOS Desktop App (SwiftUI)       │    │
│  │  Agent management, guided setup,         │    │
│  │  status dashboard, menu bar widget       │    │
│  └──────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

---

## Core Capabilities

### Encrypted Vault
Keys stored in macOS Keychain or Argon2id-encrypted file (`t=3, m=192MB, p=4`). AES-256-GCM with random salt and nonce. Atomic writes, `0600` permissions. Keys are injected at request time and never written to agent configs.

### Transparent Proxy
Full streaming support (SSE). Provider-aware routing (`/openai/...`, `/anthropic/...`, `/google/...`). Auth header injection per provider (Bearer, x-api-key, x-goog-api-key). Inbound auth-like headers stripped before forwarding.

### Budget Enforcement
Per-provider and global spend limits (daily/monthly). Cost estimation via embedded pricing map covering current models. HTTP 429 when exceeded. Real-time tracking in SQLite.

### Kill Switch
`tokfence kill` — instantly revokes all providers. Zero API calls until `tokfence unkill`. Takes effect within the current request cycle.

### Leak Detection
`tokfence watch` polls provider usage APIs (OpenAI, Anthropic) and compares against local proxy logs. Delta exceeding threshold triggers alert. Idle-window detection: if Tokfence sees no traffic but provider usage increases, something else is using the key. Optional `--auto-revoke`.

### ImmuneFence (Adaptive Security)
Runtime security layer with four components:

**Capability Tokens** — Ed25519-signed, short-lived tokens with client ID, session, scope, risk state, and expiry. Validated on every request.

**Risk State Machine** — Monotonic escalation: GREEN → YELLOW → ORANGE → RED. Each level progressively restricts what agents can do. No automatic downgrade within a session.

**Sensors** — Regex-based detection of secret patterns in request bodies, disallowed endpoints (`/v1/files`, `/v1/billing`, `/v1/admin`), and system override commands. Triggers risk escalation.

**Canary Detection** — Random marker generated per daemon process, never persisted. If it appears in a response, risk escalates to RED immediately.

### Transport Hardening
Unix Domain Socket with `0660` filesystem permissions (default: `~/.tokfence/tokfence.sock`). TCP fallback for Docker containers and external tools. Socket paths validated against macOS `sun_path` limit. Stale sockets cleaned on startup, removed on shutdown.

---

## OpenClaw Integration

```bash
tokfence vault add openai sk-...
tokfence launch
# OpenClaw starts in Docker. Keys stay in the vault.
```

What happens:
1. Tokfence generates OpenClaw config with `"apiKey": "tokfence-managed"` (dummy value).
2. `base_url` points to the Tokfence proxy via Docker host networking.
3. Container starts with health checks and auto-retry.
4. All API traffic routes through Tokfence — logged, budgeted, rate-limited.

CVE-2026-25253 exfiltrated keys from OpenClaw's `auth-profiles.json`. With Tokfence, that file contains a dummy string. Attack surface eliminated.

---

## macOS Desktop App

Native SwiftUI application with:
- Agents-first navigation (guided setup → runtime dashboard)
- One-click OpenClaw launch with Docker prerequisite checks
- Live status with debounced polling
- Provider management and vault overview
- Menu bar widget via SwiftBar integration
- Accessibility IDs for UI automation

The desktop app communicates exclusively via CLI (`tokfence status --json`, `tokfence launch status --json`). It never makes direct HTTP calls to the daemon.

---

## CLI

```bash
# Daemon
tokfence start [-d]              # start (foreground or background)
tokfence stop                    # graceful shutdown
tokfence status                  # running state, PID, address

# Vault
tokfence vault add <provider> <key|->
tokfence vault remove <provider>
tokfence vault list
tokfence vault rotate <provider> <new-key|->

# Observability
tokfence log -f --provider openai --since 1h
tokfence stats --period 7d --by model

# Budget
tokfence budget set openai 10.00 daily
tokfence budget status

# Control
tokfence revoke <provider>       # cut one provider
tokfence kill                    # cut everything
tokfence unkill                  # restore everything

# Leak Detection
tokfence watch --period 24h --interval 15m --auto-revoke

# Agent Integration
tokfence env                     # print shell exports
tokfence setup openclaw --test   # verify readiness
tokfence launch                  # start OpenClaw in Docker
```

---

## What Makes This Different

| | Raw Keys in Config | Tokfence |
|---|---|---|
| Key storage | Plaintext JSON/YAML/env | Encrypted vault (Keychain/Argon2) |
| Key exposure to agent | Full key in memory | Never — proxy injects at request time |
| Spend control | None | Per-provider daily/monthly caps |
| Visibility | Zero | Every request logged with model, tokens, cost |
| Kill switch | Delete key manually | `tokfence kill` — instant |
| Leak detection | None | Provider usage reconciliation |
| Runtime security | None | Capability tokens, risk states, canary detection |
| Transport | TCP open to all local processes | UDS with filesystem permissions |

---

## Technical Stack

- **Language:** Go (daemon, CLI, proxy, security), Swift (macOS app)
- **Crypto:** Ed25519 (capability tokens), Argon2id + AES-256-GCM (vault), macOS Keychain
- **Storage:** SQLite (logs, budgets, rate limits), TOML (config)
- **Transport:** Unix Domain Socket (primary), TCP (Docker/agent compatibility)
- **Proxy:** Full streaming/SSE, provider-aware routing, header sanitization
- **Desktop:** SwiftUI, WidgetKit, SwiftBar
- **Container:** Docker orchestration for OpenClaw with health checks

---

## Status

Production-ready for local use. Tested on macOS. All components built, tested (`go test ./...` green), and shipping.

**Implemented:**
- Encrypted vault with two backends
- Transparent streaming proxy with multi-provider routing
- Budget enforcement and rate limiting
- Kill switch and provider revocation
- Leak detection via provider usage API reconciliation
- ImmuneFence: capability tokens, risk state machine, sensors, canary detection
- Unix Domain Socket transport with filesystem permissions
- OpenClaw Docker launcher with guided setup
- Native macOS desktop app
- SwiftBar menu bar widget
- Full CLI with JSON output mode

**Next:**
- macOS code signing and notarization for public distribution
- Windows/Linux vault backends
- Additional agent framework integrations (LangChain, CrewAI, Cursor)
- Team/multi-user mode with shared policy

---

## One Line

Tokfence is the local security daemon that keeps AI agent API keys in an encrypted vault, injects them at request time, and enforces budgets, rate limits, and adaptive risk controls — so your keys never touch agent config files again.
