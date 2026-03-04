#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

cd "$ROOT_DIR"

go test ./...
go build -o bucket ./cmd/bucket

INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"

cp -f ./bucket "${INSTALL_DIR}/bucket"
chmod 0755 "${INSTALL_DIR}/bucket"

echo "Installed to ${INSTALL_DIR}/bucket"
echo "Ensure ${INSTALL_DIR} is on your PATH."
