#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec ruby "$SCRIPT_DIR/host_wol_helper.rb"
