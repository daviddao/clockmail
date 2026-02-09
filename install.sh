#!/bin/sh
set -e

# Clockmail installer — builds cm from source with version info baked in.
#
# One-liner:
#   curl -sSL https://raw.githubusercontent.com/daviddao/clockmail/main/install.sh | sh
#
# Or with a custom install directory:
#   curl -sSL https://raw.githubusercontent.com/daviddao/clockmail/main/install.sh | INSTALL_DIR=/usr/local/bin sh

REPO="github.com/daviddao/clockmail"
INSTALL_DIR="${INSTALL_DIR:-$(go env GOPATH)/bin}"
TMPDIR="${TMPDIR:-/tmp}"
CLONE_DIR="$TMPDIR/clockmail-install-$$"

cleanup() { rm -rf "$CLONE_DIR"; }
trap cleanup EXIT

# --- preflight ---
if ! command -v go >/dev/null 2>&1; then
  echo "error: go is not installed (https://go.dev/dl/)" >&2
  exit 1
fi

if ! command -v git >/dev/null 2>&1; then
  echo "error: git is not installed" >&2
  exit 1
fi

echo "cloning $REPO..."
git clone --depth 1 "https://$REPO.git" "$CLONE_DIR" 2>/dev/null

cd "$CLONE_DIR"

# --- version info from git ---
VERSION="$(git describe --tags 2>/dev/null || git rev-parse --short HEAD)"
COMMIT="$(git rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
PKG="main"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X ${PKG}.version=${VERSION}"
LDFLAGS="$LDFLAGS -X ${PKG}.commit=${COMMIT}"
LDFLAGS="$LDFLAGS -X ${PKG}.date=${DATE}"

echo "building cm ${VERSION} (${COMMIT})..."
go build -ldflags "$LDFLAGS" -o "$INSTALL_DIR/cm" ./cmd/cm

echo ""
echo "installed cm to $INSTALL_DIR/cm"
"$INSTALL_DIR/cm" --version
echo ""

# verify it's on PATH
if command -v cm >/dev/null 2>&1; then
  echo "ready. run 'cm --help' to get started."
else
  echo "note: $INSTALL_DIR is not in your PATH."
  echo "  add it:  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
