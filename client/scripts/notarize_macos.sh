#!/usr/bin/env bash
# Sign (Developer ID), notarize, staple, and package the macOS release build.
#
# macOS is distributed directly as a .dmg (Developer ID + notarization),
# NOT through the Mac App Store. Run this after:
#   flutter build macos --release
#
# Required environment variables (never hardcode credentials):
#   SIGN_IDENTITY          e.g. "Developer ID Application: ACME, Inc. (TEAMID123)"
#   APPLE_ID               Apple ID used for notarization
#   TEAM_ID                Team identifier (visible in App Store Connect)
#   APP_SPECIFIC_PASSWORD  App-specific password (appleid.apple.com -> App-Specific Passwords)
#
# Optional:
#   BUNDLE_ID              expected CFBundleIdentifier of the built app (sanity check)
#   APP_NAME               product name of the built .app (default: rfplay_client)
#   ENTITLEMENTS           path to entitlements used for signing
#
# Usage:
#   SIGN_IDENTITY="Developer ID Application: ACME, Inc. (TEAMID123)" \
#   APPLE_ID="you@example.com" TEAM_ID="TEAMID123" \
#   APP_SPECIFIC_PASSWORD="xxxx-xxxx-xxxx-xxxx" \
#   ./scripts/notarize_macos.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

SIGN_IDENTITY="${SIGN_IDENTITY:?SIGN_IDENTITY is required (e.g. \"Developer ID Application: ACME, Inc. (TEAMID123)\")}"
APPLE_ID="${APPLE_ID:?APPLE_ID is required}"
TEAM_ID="${TEAM_ID:?TEAM_ID is required}"
APP_SPECIFIC_PASSWORD="${APP_SPECIFIC_PASSWORD:?APP_SPECIFIC_PASSWORD is required}"

BUNDLE_ID="${BUNDLE_ID:-com.example.rfplayClient}"
APP_NAME="${APP_NAME:-rfplay_client}"
ENTITLEMENTS="${ENTITLEMENTS:-$ROOT/macos/Runner/Release.entitlements}"

APP_PATH="$ROOT/build/macos/Build/Products/Release/$APP_NAME.app"
WORK_DIR="$ROOT/build/macos/notarize"
ZIP_PATH="$WORK_DIR/$APP_NAME.zip"
DMG_PATH="$ROOT/build/macos/$APP_NAME.dmg"

log() { printf '\033[1;34m[notarize]\033[0m %s\n' "$*"; }

# 1. Locate the release build produced by `flutter build macos --release`.
if [ ! -d "$APP_PATH" ]; then
  log "release app not found at $APP_PATH"
  log "run: flutter build macos --release"
  exit 1
fi

# Sanity check: the built bundle identifier must match what we distribute.
BUILT_BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_PATH/Contents/Info.plist")"
if [ "$BUILT_BUNDLE_ID" != "$BUNDLE_ID" ]; then
  echo "warning: built CFBundleIdentifier is '$BUILT_BUNDLE_ID', expected '$BUNDLE_ID'" >&2
fi

# 2. Sign the app with the Developer ID Application identity and hardened
#    runtime (required for notarization). --deep also signs embedded helpers
#    (e.g. a packet tunnel extension bundled with the app).
codesign --force --deep --options runtime \
  --entitlements "$ENTITLEMENTS" \
  --sign "$SIGN_IDENTITY" \
  "$APP_PATH"

codesign --verify --deep --strict --verbose=2 "$APP_PATH"

# 3. Archive to a zip for submission to Apple's notary service.
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR"
ditto -c -k --sequesterRsrc --keepParent "$APP_PATH" "$ZIP_PATH"

# 4. Submit for notarization and block until Apple returns a verdict.
xcrun notarytool submit "$ZIP_PATH" \
  --apple-id "$APPLE_ID" \
  --team-id "$TEAM_ID" \
  --password "$APP_SPECIFIC_PASSWORD" \
  --wait

# 5. Staple the notarization ticket to the app bundle and verify.
xcrun stapler staple "$APP_PATH"
xcrun stapler validate "$APP_PATH"

# 6. Package the stapled app into a compressed DMG for distribution.
hdiutil create -volname "$APP_NAME" -srcfolder "$APP_PATH" \
  -ov -format UDZO "$DMG_PATH"

log "done: $DMG_PATH"
