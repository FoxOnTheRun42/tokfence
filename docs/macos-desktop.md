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

### Distribution build (signed + notarized)

For public distribution on macOS, Tokfence must be code signed and notarized.

Required environment (minimum):

- `TEAM_ID` (Apple developer team id)
- `CODESIGN_IDENTITY` (e.g. `Developer ID Application: ...`)
- `NOTARIZATION_PROFILE` (preconfigured `xcrun notarytool` profile)

Run:

```bash
TEAM_ID=ABCDE12345 \
CODESIGN_IDENTITY="Developer ID Application: Example" \
NOTARIZATION_PROFILE="tokfence-notary" \
make desktop-release
```

The release target runs:

1. Xcode archive with manual signing
2. Hardened runtime signatures
3. `notarytool` submit + stapling

The resulting notarized app is inside `build/desktop/TokfenceDesktop.xcarchive/Products/Applications/TokfenceDesktop.app`.

## Run

```bash
open apps/TokfenceDesktop/TokfenceDesktop.xcodeproj
```

Then run the `TokfenceDesktop` scheme from Xcode.

## Notes

- If Tokfence is not in your PATH, set the binary path in the app header.
- The app and widget are intended for macOS 14+.
- Guided setup UX spec (Agents-first): `docs/desktop-guided-setup.md`
- Icon specification pack: `docs/desktop-icons.md`
- Latest desktop smoke results: `docs/desktop-smoke-report.md`
