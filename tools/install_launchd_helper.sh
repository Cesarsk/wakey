#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP_SUPPORT_DIR="$HOME/Library/Application Support/Wakey"
HELPER_SCRIPT_SRC="$SCRIPT_DIR/host_wol_helper.rb"
START_SCRIPT_SRC="$SCRIPT_DIR/start_host_helper.sh"
PLIST_TEMPLATE="$SCRIPT_DIR/com.lucacesarano.wakey-helper.plist"
PLIST_DST="$HOME/Library/LaunchAgents/com.lucacesarano.wakey-helper.plist"
HELPER_SCRIPT_DST="$APP_SUPPORT_DIR/host_wol_helper.rb"
START_SCRIPT_DST="$APP_SUPPORT_DIR/start_host_helper.sh"
HELPER_TOKEN=${WAKEY_HOST_HELPER_TOKEN:-wakey-local-helper}

mkdir -p "$APP_SUPPORT_DIR"
mkdir -p "$HOME/Library/LaunchAgents"
cp "$HELPER_SCRIPT_SRC" "$HELPER_SCRIPT_DST"
cp "$START_SCRIPT_SRC" "$START_SCRIPT_DST"
chmod 755 "$HELPER_SCRIPT_DST" "$START_SCRIPT_DST"

sed \
  -e "s|__START_SCRIPT__|$START_SCRIPT_DST|g" \
  -e "s|__WORKDIR__|$APP_SUPPORT_DIR|g" \
  -e "s|__HELPER_TOKEN__|$HELPER_TOKEN|g" \
  "$PLIST_TEMPLATE" > "$PLIST_DST"

launchctl bootout "gui/$(id -u)/com.lucacesarano.wakey-helper" >/dev/null 2>&1 || true
launchctl unload "$PLIST_DST" >/dev/null 2>&1 || true
launchctl load "$PLIST_DST"
launchctl kickstart -k "gui/$(id -u)/com.lucacesarano.wakey-helper"

printf '%s\n' "Installed and started com.lucacesarano.wakey-helper"
