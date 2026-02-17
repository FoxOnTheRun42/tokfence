# Tokfence Desktop Guided Setup Spec (Agents-First)

## Purpose
Shift from "one-click launch" framing to a guided first-run setup that explains security decisions and removes Docker/proxy complexity for non-technical users.

User outcome:
- Understand why each requirement exists (Docker, daemon, vault, isolation).
- Finish setup without terminal usage.
- Start OpenClaw with clear success/failure signals.

## Scope
In scope:
- macOS desktop UI/UX only (`apps/TokfenceDesktop`).
- Agents page setup flow and copy.
- Step state model and acceptance tests.

Out of scope:
- Backend/CLI architecture changes.
- New runtime dependencies.
- Multi-agent orchestration beyond existing placeholders.

## Locked Decisions
1. Backend command name stays `launch`.
2. UI wording uses `Setup` / `Get Started` / `Start`.
3. First run is guided; later use is operational.
4. Agents section remains the primary landing view.

## IA
No major IA change required. Keep:
- `MY AGENTS`: `Agents`
- `PROXY`: `Overview`, `Vault`, `Activity`, `Budget`, `Providers`
- Footer: `Settings`

## UX Modes
OpenClaw card supports two macro modes:
1. `Guided Setup Mode`
2. `Runtime Mode` (stopped/starting/running/error)

Show Guided Setup Mode when any prerequisite is missing:
- Docker unavailable.
- Tokfence daemon offline.
- No supported vault key.
- Missing OpenClaw config file (`~/.tokfence/openclaw/openclaw.json`).

Show Runtime Mode when prerequisites are satisfied.

## Guided Setup Steps
Each step has: status dot, title, one-line rationale, action button, completion check.

### Step 1: Docker
- Title: `Install Docker Desktop`
- Reason: `OpenClaw runs in an isolated container for credential safety.`
- Action:
  - missing/down: `Open Docker Download`
  - installed not running: `Open Docker`
- Done when `docker info` succeeds.

### Step 2: Tokfence Daemon
- Title: `Start Tokfence Daemon`
- Reason: `Tokfence injects keys and logs all model requests.`
- Action: `Start Daemon`
- Done when daemon status is running on `127.0.0.1:9471`.

### Step 3: Vault Key
- Title: `Add API Key to Vault`
- Reason: `Keys stay encrypted and never enter the OpenClaw container.`
- Action: `Open Vault`
- Done when at least one supported provider key exists.

### Step 4: Secure Config
- Title: `Generate OpenClaw Secure Config`
- Reason: `Tokfence writes dummy-key routing config with wizard bypass.`
- Action: `Generate Config`
- Done when valid JSON exists and includes:
  - `models.mode = "merge"`
  - provider `apiKey = "tokfence-managed"`
  - wizard block with `lastRunCommand = "onboard"`.

### Step 5: Start OpenClaw
- Title: `Start OpenClaw Container`
- Reason: `Runs OpenClaw with Tokfence-secured routing.`
- Action: `Set up OpenClaw` (first run), then `Start` on later runs.
- Done when `launch status --json` is `running` and dashboard is reachable.

## Button Rules
Primary CTA label on OpenClaw card:
- prerequisites missing / first run: `Set up OpenClaw`
- setup complete + stopped: `Start`
- starting: disabled spinner
- running: `Open OpenClaw`
- error: `Retry`

Secondary runtime actions:
- `Go to OpenClaw`, `Stop`, `Restart`

Configure trigger:
- Gear icon stays top-right in card header.

## Error UX
Display priority:
1. step-level inline failure
2. card-level summary
3. toast for transient feedback only

Example messages:
- `Docker is not running. Start Docker Desktop and retry.`
- `No provider key found. Add a key in Vault.`
- `OpenClaw is unreachable on port 18789. Check container logs.`

Never render raw CLI help dumps in the main card.

## UI Model Additions
Add desktop-only types:

```swift
enum TokfenceSetupStepID: String, CaseIterable {
    case docker, daemon, vault, config, container
}

enum TokfenceSetupStepStatus {
    case pending
    case inProgress
    case complete
    case failed(message: String)
}

struct TokfenceSetupStepState: Identifiable {
    let id: TokfenceSetupStepID
    let title: String
    let reason: String
    let actionTitle: String
    let status: TokfenceSetupStepStatus
}
```

Add computed outputs in view model:
- `setupSteps`
- `isSetupComplete`
- `primarySetupActionTitle`
- `setupBlockingReason`

## Interaction Flows
First-time user:
1. Open Agents.
2. Complete setup steps in order.
3. Press `Set up OpenClaw`.
4. OpenClaw dashboard opens.

Returning user:
1. Open Agents.
2. Use runtime controls (`Start/Open/Stop/Restart`).

## Visual and Accessibility Rules
- No clipping at 1100x720.
- Step rows min height 40pt.
- Error lines max 2 lines; full text available via tooltip/copy.
- Keep existing dark theme and accent system.

## Acceptance Criteria
1. Guided setup appears only when prerequisites are missing.
2. User can complete setup without terminal use.
3. Primary CTA remains disabled until required blockers are resolved.
4. First successful setup auto-transitions to runtime mode.
5. Running mode still shows provider chips and last 3 requests.
6. Error mode provides actionable retry/configure path.
7. `Overview`, `Vault`, `Activity`, `Budget`, `Providers`, `Settings` remain unaffected.

## QA Checklist
1. Docker missing -> install/download action visible.
2. Docker stopped -> open Docker action works.
3. Daemon stopped -> daemon start action works.
4. Empty vault -> Vault handoff works.
5. Config missing -> config generation succeeds with dummy key.
6. Start success -> `running` + open dashboard action.
7. Unreachable container -> error state with retry/configure.
