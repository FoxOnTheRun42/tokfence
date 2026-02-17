# Tokfence Security Model

Tokfence is not a security boundary for all local traffic. It is a local-first control plane that removes key exfiltration risk from agent/tool config files and enforces request-time controls.

## Trust boundary

- **Protected**: provider credentials (vault, in-memory key injection, local process logs).
- **Not protected by default**: arbitrary local processes that can already execute on the same host user context.
- **Assumed trust anchors**: operator machine, local OS user, filesystem ACLs for `~/.tokfence`, and daemon process integrity.

## Transport hardening

Tokfence supports dual transport:

- `unix:/path` when `daemon.socket_path` is configured
- TCP `host:port` for tooling and Docker-compatible flows

Why both:

- UDS is safer against ambient local network peers because access is filesystem-scoped.
- TCP is required for OpenClaw/Docker integration and existing external HTTP clients.

Config defaults:

- `daemon.socket_path = "~/.tokfence/tokfence.sock"`
- `daemon.immune_enabled = true`

Startup behavior:

1. Resolve socket path and parent directory.
2. Remove stale socket if present.
3. Listen on UDS first (best-effort) and keep TCP listener as fallback.
4. Remove socket file on shutdown.

If the UDS path is overlong or invalid, startup returns a clear configuration error.

## ImmuneFence

ImmuneFence is the optional enforcement layer added around request handling.

### 1) Capability token

Each request may carry:

- `client_id`
- `session_id`
- `scope` (`proxy` or `safe`)
- `risk_state`
- `expiry` (timestamp)
- `nonce`
- signature (Ed25519)

The daemon validates token format, signature, and TTL. Invalid/missing policy can still be accepted in compatibility mode depending on `immune_enabled`, where server-generated session-scoped tokens are used.

### 2) Risk machine

State sequence is monotonic: `GREEN -> YELLOW -> ORANGE -> RED`.

- `GREEN`: default state, full routing allowed.
- `YELLOW`: elevated scrutiny, reduced token TTL, tighter checks.
- `ORANGE`: safe routes only.
- `RED`: strict deny / block behavior.

No automatic downgrade happens inside one session; restart or explicit reset is required.

### 3) Sensor layer

Runtime sensors scan request/metadata for:

- secret-like values in prompts/input
- disallowed / dangerous endpoints
- suspicious system override intents

Detected signals escalate risk and influence scope enforcement.

### 4) Canary leak detection

A random marker is generated per daemon process and never persisted.

If response output contains this marker, the request is marked suspicious and risk escalates to `RED`.

## Logging and data hygiene

Current behavior:

- Logs include route metadata (method, status, latency, token estimates).
- Response bodies are not retained as full text in normal logs.
- Runtime secrets from requests are not intentionally stored; only bounded metadata is retained.

## Claims you can make publicly

- Provider credentials do not leave the local vault.
- OpenClaw/agents receive dummy config `apiKey` values with local credential injection.
- Network transport is hardened by default with UDS preference and scoped file permissions.
- Known suspicious behavior raises risk state and can block downstream action.
