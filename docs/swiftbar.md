# SwiftBar UI

Tokfence includes a SwiftBar integration for macOS menubar usage.

## Install

```bash
tokfence widget install --refresh 20
```

Default plugin path:
`~/Library/Application Support/SwiftBar/Plugins`

## Uninstall

```bash
tokfence widget uninstall
```

## What the widget shows

- Daemon online/offline state
- Today's request count and estimated spend
- Input/output token totals
- Budget utilization bars
- Revoked provider list
- Vault provider coverage

## One-click actions

- Start / stop daemon
- Kill / unkill providers
- Open live logs
- Open stats summary
- Open Tokfence data folder

The widget output can be inspected directly with:

```bash
tokfence widget render
```
