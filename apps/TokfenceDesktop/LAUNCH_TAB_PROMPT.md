# Prompt: Redesign LaunchSectionView for Tokfence Desktop

## Context

Tokfence is a macOS SwiftUI desktop app that acts as an encrypted vault + proxy for AI agent API keys. The **Launch Tab** lets users start OpenClaw (an AI coding agent) inside Docker with zero key exposure — the container never sees real API keys.

The current LaunchSectionView works functionally but looks like a developer debug panel. It needs to become the **hero experience** of the app: one click to launch OpenClaw securely, aimed at non-technical users.

**Target user:** "Otto Normalverbraucher" — someone who wants to use OpenClaw safely but doesn't know Docker, CLI, or API key security. They should feel like they're launching a normal app.

## Architecture (do not change)

- The view receives `@ObservedObject var viewModel: TokfenceAppViewModel`
- Available ViewModel methods: `launchStart(image:name:portText:workspace:noPull:openDashboard:)`, `launchStop()`, `launchRestart()`, `launchStatus()`, `launchConfig()`, `launchLogs(follow:)`, `openOpenClawDashboard()`
- Available ViewModel state: `launchResult: TokfenceLaunchResult` (containerID, gatewayURL, gatewayToken, dashboardURL, providers, primaryModel, configPath, status, logsPreview), `launchBusy: Bool`, `launchConfigOutput: String`, `launchLogsOutput: String`
- Also available: `snapshot.running` (daemon status), `snapshot.vaultProviders` (keys in vault), `logs` (proxy request log)
- Design system: `TokfenceTheme` (colors: `.accentPrimary`, `.healthy`, `.danger`, `.warning`, `.info`, `.bgPrimary/.bgSecondary/.bgTertiary`, `.textPrimary/.textSecondary/.textTertiary`), `TokfenceCard`, `TokfenceSectionHeader`, `TokfenceEmptyState`, `TokfenceStatusDot`, `TokfenceLiveBadge`, `TokfenceMetricCard`
- Animations: `TokfenceTheme.uiAnimation`, `TokfenceTheme.uiSpring`

## Design Requirements

### 1. Hero Launch Area (top of tab, always visible)

**When NOT running:**
- Large centered area with shield/lock icon
- Headline: "Launch OpenClaw Securely"
- Subline: "Your API keys stay in the vault. The container never sees real credentials."
- **Big green button** (full-width or at least 200pt, 44pt tall, `.healthy` tint): "Launch OpenClaw"
- Disabled state with explanation if prerequisites not met

**When RUNNING:**
- Same area transforms: green pulsing dot + "OpenClaw is running securely"
- Show gateway URL as copyable pill
- Two buttons side by side: "Open Dashboard" (primary, `.info` tint) and "Stop" (bordered, `.danger` tint on hover)
- Subtle animation on state transition (opacity + scale spring)

**When LAUNCHING (busy):**
- Progress spinner replacing the button
- "Starting secure container..." text

### 2. Prerequisites Checklist (between hero and advanced, auto-checked)

A compact horizontal row of 3 status indicators:
- ✅/❌ "Daemon" — `viewModel.snapshot.running`
- ✅/❌ "Vault" — `!viewModel.snapshot.vaultProviders.isEmpty`
- ✅/❌ "Docker" — derive from whether last launch attempt failed with docker error, or optimistically show ✅

Use `TokfenceStatusDot` style (green dot = ready, red = missing). If any prerequisite fails, the Launch button should be disabled and the failing item should have an actionable hint (e.g., "Add an API key in the Vault tab" as a clickable link that switches `viewModel.selectedSection = .vault`).

### 3. Security Context Card (compact, below hero)

A single `TokfenceCard` with:
- Left: lock.shield icon in accent color
- Text: "OpenClaw normally stores API keys in plaintext JSON. CVE-2026-25253 exfiltrated them from 17,500 instances. Tokfence eliminates this: keys live in an encrypted vault, injected at request time."
- This card should feel reassuring, not alarming. Use `.bgSecondary` background, calm typography.

### 4. Live Status Panel (when running)

A `TokfenceCard` showing:
- Container ID (truncated, monospaced, copyable)
- Providers active (badges using `TokfenceProviderBadge`)
- Primary model
- Uptime (compute from launch time if available, or just show "Running")
- Last 3 proxy requests from `viewModel.logs` filtered to the launch session (reuse the compact `RequestListPanel` pattern from Dashboard)

### 5. Advanced Settings (collapsed by default)

A `DisclosureGroup("Advanced")` containing:
- Docker image TextField (default: `ghcr.io/openclaw/openclaw:latest`)
- Container name TextField (default: `tokfence-openclaw`)
- Gateway port TextField (default: `18789`)
- Workspace path TextField (default: `~/openclaw/workspace`)
- Toggle: "Skip image pull"
- Toggle: "Open dashboard after start"

These fields feed into `launchStart(...)`. Keep current binding pattern.

### 6. Container Logs (collapsed by default, below Advanced)

A `DisclosureGroup("Container Logs")` containing:
- Monospaced scrollable text view showing `viewModel.launchLogsOutput`
- "Refresh" button
- "Follow" toggle
- Max height ~200pt with scroll

## Style Notes

- Match existing app aesthetic (dark theme, monospaced data, clean cards)
- No emojis in UI — use SF Symbols only
- Green (`.healthy`) is the primary action color for launch
- Red (`.danger`) only for stop/destructive actions
- Keep the view `private struct LaunchSectionView: View` inside ContentView.swift
- Use `ScrollView` as outer wrapper for the whole tab
- Total height should work well in the ~700pt visible area without feeling cramped

## What to Replace

Replace the entire `private struct LaunchSectionView: View { ... }` block in ContentView.swift. Keep the same struct name and `@ObservedObject var viewModel: TokfenceAppViewModel` signature.
