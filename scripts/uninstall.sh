#!/usr/bin/env sh
set -eu

BIN="${HOME}/.local/bin/bucket"

if [ -f "$BIN" ]; then
  rm -f "$BIN"
  echo "Removed $BIN"
else
  echo "Not installed: $BIN"
fi

CONFIG_DIR="${HOME}/.config/bucket"
if [ -d "$CONFIG_DIR" ]; then
  rm -rf "$CONFIG_DIR"
  echo "Removed $CONFIG_DIR (including database and logs)"
fi
