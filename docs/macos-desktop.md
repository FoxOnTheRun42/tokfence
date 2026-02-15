# Native macOS UI

Tokfence ships with a native SwiftUI desktop app and WidgetKit extension.

Project path:
`apps/TokfenceDesktop`

## Components

- `TokfenceDesktop` (macOS app)
  - Dashboard for status, budgets, spend, and control actions.
  - Reads live data from `tokfence widget render --json`.
  - Writes latest snapshot to `~/.tokfence/desktop_snapshot.json`.

- `TokfenceWidgetExtension` (WidgetKit)
  - Reads the shared snapshot file.
  - Supports small and medium widget families.

## Build

```bash
make desktop-generate
make desktop-build
```

Or manually:

```bash
cd apps/TokfenceDesktop
xcodegen generate
xcodebuild -project TokfenceDesktop.xcodeproj -scheme TokfenceDesktop -configuration Debug CODE_SIGNING_ALLOWED=NO build
```

## Run

```bash
open apps/TokfenceDesktop/TokfenceDesktop.xcodeproj
```

Then run the `TokfenceDesktop` scheme from Xcode.

## Notes

- If Tokfence is not in your PATH, set the binary path in the app header.
- The app and widget are intended for macOS 14+.
