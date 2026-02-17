# Security Policy

## Scope

Tokfence is a local-first proxy and vault for API credentials.

Primary goal:
- protect provider API keys from direct exfiltration by tools/agents that should only use a local proxy URL.

Out of scope:
- preventing API abuse by a compromised process that already runs on the same machine and can reach `127.0.0.1:9471`.
- endpoint/network compromise outside the local host.

## Threat Model (Practical)

Tokfence improves security by:
- removing plaintext provider keys from agent/tool configs.
- stripping inbound auth-like headers and injecting provider auth server-side.
- enforcing local controls (budgets, revoke, kill switch, rate limits).

Tokfence does **not** provide process-level isolation by itself. Any local process with access to localhost can still send requests through the proxy unless additional OS-level controls are applied.

## ImmuneFence Security Layer

Tokfence adds a transport and request security layer ("ImmuneFence") to make local misuse harder and to fail fast on suspicious behavior.

### Capability tokens

- Each request is processed with a short-lived token in header `X-Tokfence-Capability`.
- Tokens are Ed25519-signed and include:
  - `client_id`
  - `session_id`
  - `scope` (`proxy` or `safe`)
  - `risk_state`
  - `expiry`
  - `nonce`
- If ImmuneFence is active, tokens are validated before proxying; invalid/expired/forged tokens are rejected with 403.
- In non-strict local mode, the daemon can issue ephemeral internal tokens when missing a client token.

### Risk state machine

Tokfence tracks runtime risk with four states:

- `GREEN`: normal operation.
- `YELLOW`: increased scrutiny; request scope is reduced (shorter capability TTL and stricter checks).
- `ORANGE`: safe-only request handling for read-like routes.
- `RED`: request actions are blocked until the session is restarted.

Risk increases from:
- secret-pattern sightings in request body/input
- disallowed endpoint usage
- system-override commands in input
- canary leak evidence in responses

### Canary leak detection

At startup, Tokfence generates a random canary marker in memory only.
If the marker appears in a response stream/body, the request is flagged and risk escalates to `RED`.

### Dual transport mode

- Daemon advertises both:
  - Unix socket (when `daemon.socket_path` is configured)
  - TCP fallback (legacy and container compatibility)
- Default socket path is `~/.tokfence/tokfence.sock` and is created with parent permissions `0700`, socket perms `0660`, then removed on shutdown.

## OpenClaw Integration Security Properties

When `tokfence launch` is used to run OpenClaw:

1. **No API keys in the container.** OpenClaw's config contains `"apiKey": "tokfence-managed"` (a dummy value). Real keys never leave Tokfence's vault.
2. **CVE-2026-25253 impact reduced to zero.** This CVE exfiltrated API keys from OpenClaw's `auth-profiles.json`. With Tokfence, that file either doesn't exist or contains only the dummy value.
3. **All API traffic is logged.** Every request from the container passes through Tokfence's proxy. Visible in `tokfence log`.
4. **Budget caps apply.** `tokfence budget set anthropic daily 10.00` limits container spend. Exceeded budgets return HTTP 429.
5. **Kill switch works instantly.** `tokfence kill` revokes all providers. The container cannot make API calls until restored.
6. **Key leak detection.** `tokfence watch` monitors for keys appearing where they shouldn't.

What Tokfence does NOT protect against:
- A compromised process on the host can still make requests through the tokfence proxy.
- Tokfence prevents key exfiltration (stealing the key), but does not prevent key misuse.
- Budget caps and rate limits are the containment layer for local misuse risk.

## Cryptography

Encrypted file vault backend (`~/.tokfence/vault.enc`):
- KDF: Argon2id (`t=3`, `m=196608 KiB`, `p=4`, key length `32`).
- Encryption: AES-256-GCM with random 16-byte salt and 12-byte nonce.
- File write path: atomic temp file + rename, mode `0600`, parent dir `0700`.

macOS default vault backend:
- Keychain-backed storage.

Notes:
- Runtime memory protections are best effort in Go; zeroization is attempted but cannot be treated as a hard guarantee against all memory forensics.

## Reporting a Vulnerability

Please report vulnerabilities privately via GitHub Security Advisories:

- Go to the repository **Security** tab.
- Use **Report a vulnerability** (private disclosure).

If private reporting is unavailable, open an issue with minimal detail and request a private channel for full reproduction steps.

## Disclosure Process

- Acknowledge report within 72 hours.
- Triage and reproduce.
- Ship a fix and document impact/remediation.
- Credit reporter (optional) after patch release.
