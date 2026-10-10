#!/usr/bin/env bash
#
# mas-package.sh — build and package the Mac App Store variant of co-shell.app.
#
# This is the App Store counterpart of build-app.sh: it builds the sandboxed
# bundle (app-sandbox + the wide permission series, helpers via inherit) and
# wraps it in a signed installer package, which is what App Store Connect
# accepts for a Mac app.
#
# Usage:
#   app/mas-package.sh [OUTPUT_DIR]        # default: <repo>/work/mas
#
# Environment:
#   CO_SHELL_SIGN_ID        app signing identity
#                           (default: "Apple Distribution: Shuang Liang (5BZBX3DL83)")
#   CO_SHELL_INSTALLER_ID   installer signing identity
#                           (default: "3rd Party Mac Developer Installer: Shuang Liang (5BZBX3DL83)")
#   CO_SHELL_MAS_PROFILE    Mac App Store .provisionprofile to embed in the app
#
# Prerequisites (see docs/macos-appstore-release.md):
#   - an "Apple Distribution" certificate in the login keychain
#   - a "3rd Party Mac Developer Installer" certificate
#   - a "Mac App Store" provisioning profile for com.coshell.app.mas
# The script checks the two certificates up front and stops with a clear message
# when they are missing -- it never falls back to a development identity, so a
# package produced here is always one that can be uploaded.
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

OUT_DIR="${1:-$REPO_ROOT/work/mas}"

APP_ID="${CO_SHELL_SIGN_ID:-Apple Distribution: Shuang Liang (5BZBX3DL83)}"
INSTALLER_ID="${CO_SHELL_INSTALLER_ID:-3rd Party Mac Developer Installer: Shuang Liang (5BZBX3DL83)}"

VERSION="$(sed -n 's/^const version = "\(.*\)"/\1/p' "$REPO_ROOT/main.go" | head -1)"
if [ -z "$VERSION" ]; then
  echo "error: cannot read version from main.go" >&2
  exit 1
fi

# --- preflight: both identities must exist ------------------------------------
if ! security find-identity -v 2>/dev/null | grep -qF "$APP_ID"; then
  echo "error: app signing identity not found in the keychain:" >&2
  echo "         $APP_ID" >&2
  echo "       Create it in Xcode > Settings > Accounts > Manage Certificates" >&2
  echo "       (Apple Distribution), then re-run. See docs/macos-appstore-release.md §2." >&2
  exit 1
fi
if ! security find-identity -v 2>/dev/null | grep -qF "$INSTALLER_ID"; then
  echo "error: installer signing identity not found in the keychain:" >&2
  echo "         $INSTALLER_ID" >&2
  echo "       Create it in Xcode > Settings > Accounts > Manage Certificates" >&2
  echo "       (Mac Installer Distribution), then re-run. See docs/macos-appstore-release.md §2." >&2
  exit 1
fi

echo "==> building sandboxed bundle v$VERSION into $OUT_DIR"
CO_SHELL_MAS=1 CO_SHELL_SIGN_ID="$APP_ID" "$REPO_ROOT/app/build-app.sh" "$OUT_DIR"

APP_BUNDLE="$OUT_DIR/co-shell.app"
PKG="$OUT_DIR/co-shell-$VERSION.pkg"

# Refuse to package something that is not actually the App Store build: a wrong
# bundle id or a missing sandbox entitlement would only be caught by App Store
# Connect, long after the fact.
BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_BUNDLE/Contents/Info.plist")"
if [ "$BUNDLE_ID" != "com.coshell.app.mas" ]; then
  echo "error: unexpected bundle id '$BUNDLE_ID' (expected com.coshell.app.mas)" >&2
  exit 1
fi
if ! codesign -d --entitlements - "$APP_BUNDLE" 2>/dev/null | grep -q "com.apple.security.app-sandbox"; then
  echo "error: the app bundle is not sandboxed; refusing to package" >&2
  exit 1
fi

echo "==> packaging $PKG"
rm -f "$PKG"

# Note: productbuild prints "write: Permission denied" and stores a few "
# AppleDouble entries in the payload on locally built bundles. They come from
# the system-managed com.apple.provenance extended attribute, which cannot be
# stripped (verified: xattr -cr and ditto --noextattr both leave it in place).
# Harmless in practice; see docs/macos-appstore-release.md §7.
productbuild --component "$APP_BUNDLE" /Applications --sign "$INSTALLER_ID" "$PKG"

echo "==> done: $PKG"
cat <<EOF

Upload it to App Store Connect with one of:
  xcrun altool --upload-app -f "$PKG" --type macos -u <apple-id> -p <app-specific-password>
  Transporter.app  (drag the .pkg in)

Full checklist and troubleshooting: docs/macos-appstore-release.md
EOF
