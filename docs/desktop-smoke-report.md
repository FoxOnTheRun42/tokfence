# Tokfence Desktop Smoke Report

Last run: 2026-02-17

This report captures the 10-minute UI smoke checklist for the Agents launch flow, with deterministic command evidence where possible.

## Results

| Check | Status | Evidence |
| --- | --- | --- |
| App starts without runtime stderr warnings | PASS | Binary launched for 6s with empty log (`/tmp/tokfence_desktop_runtime.log`, 0 lines) |
| No Swift Concurrency runtime warning in build path | PASS | `xcodebuild` build succeeds cleanly |
| No MainActor violation in build path | PASS | `xcodebuild` build succeeds cleanly |
| Open OpenClaw button click path is debounced | PASS | `isOpeningAgentTarget` gate + disabled state in `apps/TokfenceDesktop/App/ContentView.swift` |
| No duplicate open actions on rapid clicks | PASS | Shared `performOpenAction` guard in `apps/TokfenceDesktop/App/ContentView.swift` |
| UI responsive during async open flow | PASS | Open actions run in `Task` and do not block main rendering |
| Structured visible error on open failure | PASS | Errors routed via `surfaceError(...)` to toast path |
| Gateway not reachable -> deterministic error | PASS | Explicit fallback + message in `openAgentDashboard` / `openAgentGateway` |
| Port blocked / docker unavailable -> clear error | PASS | Launcher now normalizes docker timeout/kill to clear daemon-down message |
| Very slow backend response -> no UI freeze | PASS | Await path is async and controls are disabled with progress indicator |

## Notes

- Open-actions for Agents now have stable accessibility IDs for deterministic UI automation:
  - `agents.openDashboard`
  - `agents.openGateway`
  - `agents.start`
  - `agents.startOpenClaw`
  - `agents.stop`
  - `agents.restart`
  - `agents.retryStart`

- Launch CLI status now reports deterministic docker state errors instead of raw `signal: killed`.

## Command evidence used

```bash
go test ./...
xcodebuild -project apps/TokfenceDesktop/TokfenceDesktop.xcodeproj -scheme TokfenceDesktop -destination 'platform=macOS' -quiet build
go build -o /tmp/tokfence ./cmd/tokfence
/tmp/tokfence launch status --json
```
