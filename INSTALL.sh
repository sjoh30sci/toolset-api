#!/bin/bash
# Quick install script for the toolset CLI.
#
#   curl -fsSL https://raw.githubusercontent.com/yourusername/toolset-api/main/INSTALL.sh | bash
#   curl -fsSL .../INSTALL.sh | bash -s -- v0.1.0   # pin a version
#
set -euo pipefail

REPO="yourusername/toolset-api"
VERSION="${1:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Resolve "latest" to the newest release tag via the GitHub API.
if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -oE '"tag_name": *"[^"]+"' | head -n1 | cut -d'"' -f4)"
fi

if [ -z "$VERSION" ]; then
  echo "could not determine release version" >&2
  exit 1
fi

# Release archives are named toolset_<version>_<os>_<arch>.(tar.gz|zip).
# The version in the archive name omits the leading "v".
NUM_VERSION="${VERSION#v}"
EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"
ARCHIVE="toolset_${NUM_VERSION}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $URL ..."
curl -fsSL "$URL" -o "$TMP/$ARCHIVE"

echo "Extracting ..."
if [ "$EXT" = "zip" ]; then
  unzip -q "$TMP/$ARCHIVE" -d "$TMP"
else
  tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
fi

echo "Installing to $INSTALL_DIR ..."
install -m 0755 "$TMP/toolset" "$INSTALL_DIR/toolset" 2>/dev/null \
  || sudo install -m 0755 "$TMP/toolset" "$INSTALL_DIR/toolset"

echo "Installed: $($INSTALL_DIR/toolset --version)"
