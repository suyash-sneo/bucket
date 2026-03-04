#!/usr/bin/env sh
set -eu

BIN="${BUCKET_INSTALL_DIR:-${HOME}/.local/bin}/bucket"
CONFIG_DIR="${HOME}/.config/bucket"
KEEP_DATA="${BUCKET_KEEP_DATA:-0}"

if [ -f "$BIN" ]; then
  rm -f "$BIN"
  echo "Removed $BIN"
else
  echo "Not installed: $BIN"
fi

if [ "$KEEP_DATA" = "1" ]; then
  echo "Keeping ${CONFIG_DIR} because BUCKET_KEEP_DATA=1"
  exit 0
fi

if [ -d "$CONFIG_DIR" ]; then
  rm -rf "$CONFIG_DIR"
  echo "Removed $CONFIG_DIR (including database and logs)"
else
  echo "No config directory found: $CONFIG_DIR"
fi
