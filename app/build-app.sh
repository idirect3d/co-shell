#!/usr/bin/env bash
#
# build-app.sh — build the double-clickable co-shell.app bundle.
#
# The bundle embeds the co-shell and co-shell-hub binaries (the same artifacts
# the Go release uses) plus a thin Swift shell that launches hub and shows its
# Web UI in a WKWebView window. No Go source is modified.
#
# Usage:
#   app/build-app.sh [OUTPUT_DIR]      # default: <repo>/work
#   CO_SHELL_MAS=1 app/build-app.sh [OUTPUT_DIR]
#
# Environment:
#   CO_SHELL_SIGN_ID      codesign identity (default: the local Apple Development cert)
#   CO_SHELL_MAS=1        build the sandboxed Mac App Store variant (FIX-630)
#   CO_SHELL_MAS_PROFILE  Mac App Store .provisionprofile to embed (MAS mode only)
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MACOS_DIR="$REPO_ROOT/app/macos"

OUT_DIR="${1:-$REPO_ROOT/work}"
APP_BUNDLE="$OUT_DIR/co-shell.app"
BIN_DIR="$APP_BUNDLE/Contents/Resources/bin"

# Sandbox / Mac App Store channel (FIX-630). In this channel the embedded
# helpers are signed with helper.entitlements (app-sandbox + inherit) and the
# app with app.entitlements (the wide permission tier), both under a hardened
# runtime, and the bundle id becomes com.coshell.app.mas so the App Store build
# can coexist with the Developer ID build.
MAS="${CO_SHELL_MAS:-0}"
if [ "$MAS" = "1" ]; then
  SIGN_ID="${CO_SHELL_SIGN_ID:-Apple Distribution: Shuang Liang (5BZBX3DL83)}"
  BUNDLE_ID="com.coshell.app.mas"
else
  SIGN_ID="${CO_SHELL_SIGN_ID:-Apple Development: Shuang Liang (4DXG4X72ZG)}"
  BUNDLE_ID="com.coshell.app"
fi

VERSION="$(sed -n 's/^const version = "\(.*\)"/\1/p' "$REPO_ROOT/main.go" | head -1)"
BUILD="$(sed -n 's/^const build = "\(.*\)"/\1/p' "$REPO_ROOT/main.go" | head -1)"
if [ -z "$VERSION" ] || [ -z "$BUILD" ]; then
  echo "error: cannot read version/build from main.go" >&2
  exit 1
fi

echo "==> building co-shell.app v$VERSION (build $BUILD) into $OUT_DIR"

# --- 1. bundle skeleton -------------------------------------------------------
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS" "$BIN_DIR"

# --- 2. compile Go binaries straight into the bundle --------------------------
(
  cd "$REPO_ROOT"
  go build -o "$BIN_DIR/co-shell" .
  go build -o "$BIN_DIR/co-shell-hub" ./cmd/co-shell-hub
)

# --- 3. compile the Swift shell ----------------------------------------------
# -parse-as-library: single-file compilation otherwise treats the file as a
# script (top-level code), which conflicts with the @main entry point.
swiftc -O -parse-as-library -o "$APP_BUNDLE/Contents/MacOS/co-shell-app" \
  "$MACOS_DIR/CoShellApp.swift" \
  -framework Cocoa -framework WebKit

# --- 4. Info.plist (version/build come from main.go) --------------------------
cp "$MACOS_DIR/Info.plist" "$APP_BUNDLE/Contents/Info.plist"
plutil -replace CFBundleShortVersionString -string "$VERSION" "$APP_BUNDLE/Contents/Info.plist"
plutil -replace CFBundleVersion -string "$BUILD" "$APP_BUNDLE/Contents/Info.plist"
plutil -replace CFBundleIdentifier -string "$BUNDLE_ID" "$APP_BUNDLE/Contents/Info.plist"
if [ "$MAS" = "1" ]; then
  # The Mac App Store provisioning profile must be sealed inside the bundle
  # before signing, and is signed along with it.
  if [ -n "${CO_SHELL_MAS_PROFILE:-}" ] && [ -f "${CO_SHELL_MAS_PROFILE}" ]; then
    cp "$CO_SHELL_MAS_PROFILE" "$APP_BUNDLE/Contents/embedded.provisionprofile"
  else
    echo "warning: CO_SHELL_MAS_PROFILE not set or missing; signing without an embedded profile" >&2
  fi
fi

# --- 5. app icon -------------------------------------------------------------
cp "$MACOS_DIR/AppIcon.icns" "$APP_BUNDLE/Contents/Resources/AppIcon.icns"

# --- 6. sign inside-out ------------------------------------------------------
# Helpers are signed first, then the app bundle. In MAS mode each side gets its
# own entitlements file and a hardened runtime; the helpers deliberately declare
# no rights of their own (app-sandbox + inherit only) -- see "spike v1" in
# docs/macos-appstore-sandbox-design.md: signing a helper with app-sandbox and
# launching it directly dies instantly with SIGTRAP.
if [ "$MAS" = "1" ]; then
  codesign --force --options runtime \
    --entitlements "$MACOS_DIR/helper.entitlements" --sign "$SIGN_ID" "$BIN_DIR/co-shell"
  codesign --force --options runtime \
    --entitlements "$MACOS_DIR/helper.entitlements" --sign "$SIGN_ID" "$BIN_DIR/co-shell-hub"
  codesign --force --options runtime \
    --entitlements "$MACOS_DIR/app.entitlements" --sign "$SIGN_ID" "$APP_BUNDLE"
else
  codesign --force --sign "$SIGN_ID" "$BIN_DIR/co-shell"
  codesign --force --sign "$SIGN_ID" "$BIN_DIR/co-shell-hub"
  codesign --force --sign "$SIGN_ID" "$APP_BUNDLE"
fi

echo "==> done: $APP_BUNDLE (v$VERSION build $BUILD)"
