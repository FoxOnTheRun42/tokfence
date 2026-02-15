# Vault Subsystem

## What it does
Stores API keys locally and injects them into proxied requests without exposing secrets to callers.

## Structure
- `internal/vault/vault.go`: interface and backend selection.
- `internal/vault/keychain_darwin.go`: macOS Keychain backend.
- `internal/vault/encrypted_file.go`: AES-256-GCM encrypted file backend (Argon2 key derivation).

## Key decisions
- macOS defaults to Keychain; file backend is available and used on non-darwin or by override.
- File backend uses `TOKFENCE_VAULT_PASSPHRASE` to derive encryption keys.
- Vault file and data directory permissions are hardened (`0600` and `0700`).

## Limitations
- Keychain list operation checks known providers only.
- File backend requires passphrase availability in non-interactive contexts.
