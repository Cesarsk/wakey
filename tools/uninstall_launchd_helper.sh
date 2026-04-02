#!/bin/sh
set -eu

LABEL="com.lucacesarano.wakey-helper"
PLIST_DST="$HOME/Library/LaunchAgents/$LABEL.plist"
APP_SUPPORT_DIR="$HOME/Library/Application Support/Wakey"

launchctl bootout "gui/$(id -u)/$LABEL" >/dev/null 2>&1 || true
rm -f "$PLIST_DST"
rm -f "$APP_SUPPORT_DIR/host_wol_helper.rb" "$APP_SUPPORT_DIR/start_host_helper.sh"
rmdir "$APP_SUPPORT_DIR" >/dev/null 2>&1 || true

printf '%s\n' "Uninstalled $LABEL"
