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
