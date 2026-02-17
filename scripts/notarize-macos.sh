#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_DIR="$ROOT_DIR/apps/TokfenceDesktop"
BUILD_DIR="${DESKTOP_DIST_DIR:-$ROOT_DIR/build/desktop}"
ARCHIVE_PATH="$BUILD_DIR/TokfenceDesktop.xcarchive"
DERIVED_DIR="$BUILD_DIR/derived"
NOTARY_ZIP="$BUILD_DIR/tokfence-desktop-macos.zip"

TEAM_ID="${TEAM_ID:-}"
CODESIGN_IDENTITY="${CODESIGN_IDENTITY:-}"
NOTARIZATION_PROFILE="${NOTARIZATION_PROFILE:-}"
SCHEME_NAME="TokfenceDesktop"

if [[ -z "$TEAM_ID" ]]; then
  echo "TEAM_ID is required for signed builds (Apple team identifier)" >&2
  exit 1
fi

if [[ -z "$CODESIGN_IDENTITY" ]]; then
  echo "CODESIGN_IDENTITY is required for signing (e.g. 'Developer ID Application: ...')" >&2
  exit 1
fi

if [[ -z "$NOTARIZATION_PROFILE" ]]; then
  echo "NOTARIZATION_PROFILE is required (xcrun notarytool keychain profile name)" >&2
  exit 1
fi

mkdir -p "$BUILD_DIR"

cd "$PROJECT_DIR"

xcodebuild \
  -project TokfenceDesktop.xcodeproj \
  -scheme "$SCHEME_NAME" \
  -configuration Release \
  -derivedDataPath "$DERIVED_DIR" \
  -archivePath "$ARCHIVE_PATH" \
  CODE_SIGN_STYLE=Manual \
  CODE_SIGN_IDENTITY="$CODESIGN_IDENTITY" \
  DEVELOPMENT_TEAM="$TEAM_ID" \
  CODE_SIGNING_ALLOWED=YES \
  archive

APP_PATH="$ARCHIVE_PATH/Products/Applications/$SCHEME_NAME.app"
WIDGET_EXT="${APP_PATH}/Contents/PlugIns/TokfenceWidgetExtension.appex"

codesign --force --timestamp --options runtime --sign "$CODESIGN_IDENTITY" "$APP_PATH/Contents/Frameworks" 2>/dev/null || true
codesign --force --timestamp --options runtime --sign "$CODESIGN_IDENTITY" "$WIDGET_EXT"
codesign --force --timestamp --options runtime --sign "$CODESIGN_IDENTITY" "$APP_PATH"

codesign --verify --strict --verbose=2 "$APP_PATH"

rm -f "$NOTARY_ZIP"
ditto -c -k --keepParent "$APP_PATH" "$NOTARY_ZIP"

xcrun notarytool submit "$NOTARY_ZIP" \
  --keychain-profile "$NOTARIZATION_PROFILE" \
  --wait

xcrun stapler staple "$APP_PATH"
xcrun stapler validate "$APP_PATH"

echo "Notarized artifact: $APP_PATH"
echo "Notary zip: $NOTARY_ZIP"
