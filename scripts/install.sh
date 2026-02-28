#!/usr/bin/env sh
set -eu

REPO="gelleson/autoport"
BINARY="autoport"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-latest}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar
need_cmd uname

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "error: unsupported OS '$OS'. Use scripts/install.ps1 on Windows." >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "error: unsupported architecture '$ARCH'" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  if [ -z "$VERSION" ]; then
    echo "error: failed to resolve latest release tag" >&2
    exit 1
  fi
fi

ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

echo "Downloading $URL"
curl -fL "$URL" -o "$TMP_DIR/$ARCHIVE"
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

TARGET="$INSTALL_DIR/$BINARY"
SOURCE="$TMP_DIR/$BINARY"

if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true
fi

if [ ! -w "$INSTALL_DIR" ]; then
  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 0755 "$SOURCE" "$TARGET"
  else
    FALLBACK_DIR="$HOME/.local/bin"
    mkdir -p "$FALLBACK_DIR"
    install -m 0755 "$SOURCE" "$FALLBACK_DIR/$BINARY"
    echo "Installed to $FALLBACK_DIR/$BINARY"
    echo "Add '$FALLBACK_DIR' to PATH if needed."
    exit 0
  fi
else
  install -m 0755 "$SOURCE" "$TARGET"
fi

echo "Installed $BINARY $VERSION to $TARGET"
