#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-run}"
APP_NAME="ClaudexDesktop"
BUNDLE_ID="com.claudex.ClaudexDesktop"
MIN_SYSTEM_VERSION="13.0"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
APP_BUNDLE="$DIST_DIR/$APP_NAME.app"
APP_CONTENTS="$APP_BUNDLE/Contents"
APP_MACOS="$APP_CONTENTS/MacOS"
APP_RESOURCES="$APP_CONTENTS/Resources"
APP_BINARY="$APP_MACOS/$APP_NAME"
SERVER_BINARY="$APP_RESOURCES/claudex-server"
INFO_PLIST="$APP_CONTENTS/Info.plist"

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "ClaudexDesktop packaging requires macOS" >&2
    exit 1
fi

pkill -x "$APP_NAME" >/dev/null 2>&1 || true

rm -rf "$APP_BUNDLE"
mkdir -p "$APP_MACOS" "$APP_RESOURCES"

CGO_ENABLED=0 go build -trimpath -o "$APP_BINARY" ./cmd/claudexdesktop
CGO_ENABLED=0 go build -trimpath -o "$SERVER_BINARY" ./cmd/claudex
cp "$ROOT_DIR/claudex.example.yaml" "$APP_RESOURCES/claudex.example.yaml"
chmod 755 "$APP_BINARY" "$SERVER_BINARY"

apply_patch_info_plist() {
    /usr/bin/plutil -convert xml1 -o "$INFO_PLIST" - <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDisplayName</key>
  <string>$APP_NAME</string>
  <key>CFBundleExecutable</key>
  <string>$APP_NAME</string>
  <key>CFBundleIdentifier</key>
  <string>$BUNDLE_ID</string>
  <key>CFBundleName</key>
  <string>$APP_NAME</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>0.1.0</string>
  <key>CFBundleVersion</key>
  <string>0.1.0</string>
  <key>LSMinimumSystemVersion</key>
  <string>$MIN_SYSTEM_VERSION</string>
  <key>NSPrincipalClass</key>
  <string>NSApplication</string>
</dict>
</plist>
PLIST
}

apply_patch_info_plist

open_app() {
    /usr/bin/open -n "$APP_BUNDLE"
}

case "$MODE" in
run)
    open_app
    ;;
--debug|debug)
    lldb -- "$APP_BINARY"
    ;;
--logs|logs)
    open_app
    tail -f "$HOME/.claudex/desktop.log"
    ;;
--telemetry|telemetry)
    open_app
    /usr/bin/log stream --info --style compact --predicate "process == \"$APP_NAME\""
    ;;
--verify|verify)
    open_app
    for _ in $(seq 1 60); do
        if pgrep -x "$APP_NAME" >/dev/null 2>&1; then
            exit 0
        fi
        sleep 0.25
    done
    echo "$APP_NAME did not start" >&2
    exit 1
    ;;
*)
    echo "usage: $0 [run|--debug|--logs|--telemetry|--verify]" >&2
    exit 2
    ;;
esac
