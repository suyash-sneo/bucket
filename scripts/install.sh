#!/usr/bin/env sh
set -eu

REPO="${BUCKET_REPO:-suyash-sneo/bucket}"
INSTALL_DIR="${BUCKET_INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${BUCKET_VERSION:-}"

OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" != "Darwin" ]; then
  echo "This installer currently supports macOS only." >&2
  exit 1
fi

case "$ARCH" in
  arm64|aarch64)
    ARCH="arm64"
    ;;
  x86_64|amd64)
    ARCH="amd64"
    ;;
  *)
    echo "Unsupported macOS architecture: $ARCH" >&2
    exit 1
    ;;
esac

if [ -z "$VERSION" ]; then
  # /releases/latest excludes prereleases and returns 404 when only prereleases exist.
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | awk -F'"' '/tag_name/ {print $4; exit}' || true)"
fi

if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=1" | awk -F'"' '/tag_name/ {print $4; exit}')"
fi

if [ -z "$VERSION" ]; then
  echo "Failed to detect a release version from GitHub for ${REPO}." >&2
  echo "Set BUCKET_VERSION explicitly, e.g. BUCKET_VERSION=v0.0.1." >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

mkdir -p "$INSTALL_DIR"
SELECTED_ASSET=""
for CANDIDATE in \
  "bucket-${VERSION}-darwin-${ARCH}.tar.gz" \
  "bucket-${VERSION}-${ARCH}.tar.gz"
do
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${CANDIDATE}"
  if curl -fsSL "$URL" -o "${TMP_DIR}/${CANDIDATE}" 2>/dev/null; then
    SELECTED_ASSET="$CANDIDATE"
    break
  fi
done

if [ -z "$SELECTED_ASSET" ]; then
  echo "No matching macOS asset found for ${REPO} ${VERSION} (${ARCH})." >&2
  echo "Expected one of:" >&2
  echo "  bucket-${VERSION}-darwin-${ARCH}.tar.gz" >&2
  echo "  bucket-${VERSION}-${ARCH}.tar.gz" >&2
  exit 1
fi

tar -xzf "${TMP_DIR}/${SELECTED_ASSET}" -C "$TMP_DIR"

SELECTED_BINARY=""
for CANDIDATE_BIN in \
  "bucket-${VERSION}-darwin-${ARCH}" \
  "bucket-${VERSION}-${ARCH}"
do
  if [ -f "${TMP_DIR}/${CANDIDATE_BIN}" ]; then
    SELECTED_BINARY="$CANDIDATE_BIN"
    break
  fi
done

if [ -z "$SELECTED_BINARY" ]; then
  echo "Downloaded asset ${SELECTED_ASSET} but expected binary was not found." >&2
  exit 1
fi

mv -f "${TMP_DIR}/${SELECTED_BINARY}" "${INSTALL_DIR}/bucket"
chmod 0755 "${INSTALL_DIR}/bucket"

echo "Installed ${REPO} ${VERSION} to ${INSTALL_DIR}/bucket"
echo "If needed, add ${INSTALL_DIR} to PATH."
