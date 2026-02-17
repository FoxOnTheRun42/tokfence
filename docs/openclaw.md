# OpenClaw Integration

Tokfence can start OpenClaw in Docker with zero keys in container state. You keep API keys in Tokfence vault and let the local proxy inject credentials at request time.

## One-Click Launch

### Prerequisites

- Docker Desktop (or Docker Engine + CLI)
- At least one API key stored in Tokfence vault

### Quick flow

```bash
tokfence vault add anthropic sk-ant-...
tokfence start -d
tokfence launch
```

OpenClaw starts in Docker and points all API traffic to `tokfence` proxy.

### Managing OpenClaw

- `tokfence launch status` — show container state and config-backed state.
- `tokfence launch logs -f` — tail OpenClaw logs.
- `tokfence launch restart` — regenerate config and restart with updated vault/providers.
- `tokfence launch stop` — stop container and remove it.

### Adding/removing providers

Add keys as needed:

```bash
tokfence vault add openai sk-proj-...
tokfence launch restart
```

### Budget + controls in OpenClaw mode

```bash
tokfence budget set anthropic daily 10.00
tokfence kill
```

`tokfence launch` does not bypass your global/per-provider budget caps or kill switch.

## Manual Integration

If you already run OpenClaw yourself, configure its provider entry to point to the local Tokfence proxy and use a dummy key.

Important behavior:
- `baseUrl` is set to `http://host.docker.internal:9471/<provider>`
- `apiKey` is always `tokfence-managed`
- provider routes are resolved by Tokfence and credentials are injected locally

Use this with existing OpenClaw installs:

```bash
tokfence setup openclaw --provider anthropic --test
```

### Docker networking

- macOS/Windows: `host.docker.internal` is available by default.
- Linux: `--add-host=host.docker.internal:host-gateway` is added automatically.

### CVE context

OpenClaw-related exposure issues (historical incidents where API keys could leak to local files like `auth-profiles.json`) are reduced because OpenClaw receives only `"tokfence-managed"` and real credentials stay in Tokfence vault.
