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
#
# Environment:
#   CO_SHELL_SIGN_ID   codesign identity (default: the local Apple Development cert)
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MACOS_DIR="$REPO_ROOT/app/macos"

OUT_DIR="${1:-$REPO_ROOT/work}"
APP_BUNDLE="$OUT_DIR/co-shell.app"
BIN_DIR="$APP_BUNDLE/Contents/Resources/bin"

SIGN_ID="${CO_SHELL_SIGN_ID:-Apple Development: Shuang Liang (4DXG4X72ZG)}"

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

# --- 5. app icon -------------------------------------------------------------
cp "$MACOS_DIR/AppIcon.icns" "$APP_BUNDLE/Contents/Resources/AppIcon.icns"

# --- 6. sign inside-out ------------------------------------------------------
codesign --force --sign "$SIGN_ID" "$BIN_DIR/co-shell"
codesign --force --sign "$SIGN_ID" "$BIN_DIR/co-shell-hub"
codesign --force --sign "$SIGN_ID" "$APP_BUNDLE"

echo "==> done: $APP_BUNDLE (v$VERSION build $BUILD)"
