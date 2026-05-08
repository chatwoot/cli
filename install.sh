#!/usr/bin/env sh
# install.sh — download a chatwoot-cli release binary into a local bin dir.
#
# Usage:
#   curl -fsSL https://chwt.app/install-cli | sh
#
# Environment:
#   CHATWOOT_VERSION   pin a specific version, e.g. "v0.2.1" (default: latest)
#   CHATWOOT_INSTALL_DIR  install location (default: $HOME/.local/bin)

set -eu

REPO="chatwoot/cli"
BINARY="chatwoot"
INSTALL_DIR="${CHATWOOT_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${CHATWOOT_VERSION:-latest}"

err() { printf 'install: %s\n' "$*" >&2; exit 1; }
log() { printf 'install: %s\n' "$*"; }

# ---------------------------------------------------------------------------
# Detect platform
# ---------------------------------------------------------------------------
case "$(uname -s)" in
  Darwin) os=Darwin ;;
  Linux)  os=Linux ;;
  *) err "unsupported OS: $(uname -s) (Windows users: download from https://github.com/$REPO/releases)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=x86_64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) err "unsupported arch: $(uname -m)" ;;
esac

# ---------------------------------------------------------------------------
# Resolve version (latest by default)
# ---------------------------------------------------------------------------
if [ "$VERSION" = "latest" ]; then
  log "resolving latest version"
  # Follow redirect on /releases/latest to capture the tag without auth.
  VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/$REPO/releases/latest" 2>/dev/null \
    | sed -n 's|.*/tag/\(v[^/]*\).*|\1|p')
  [ -n "$VERSION" ] || err "could not resolve latest version (check network)"
fi
case "$VERSION" in
  v*) ;;
  *) VERSION="v$VERSION" ;;
esac
ver_clean="${VERSION#v}"

# ---------------------------------------------------------------------------
# Build URLs and download
# ---------------------------------------------------------------------------
asset="${BINARY}_${ver_clean}_${os}_${arch}.tar.gz"
asset_url="https://github.com/$REPO/releases/download/${VERSION}/${asset}"
checksum_url="https://github.com/$REPO/releases/download/${VERSION}/checksums.txt"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

log "downloading ${asset_url}"
if ! curl -fsSL "$asset_url" -o "$tmp/$asset"; then
  err "failed to download ${asset_url} (no release for ${os}/${arch}?)"
fi

# ---------------------------------------------------------------------------
# Verify checksum (sha256)
# ---------------------------------------------------------------------------
if curl -fsSL "$checksum_url" -o "$tmp/checksums.txt" 2>/dev/null; then
  expected=$(grep " ${asset}\$" "$tmp/checksums.txt" | awk '{print $1}')
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
    else
      log "warning: no sha256 tool found — skipping checksum verification"
      actual=""
    fi
    if [ -n "$actual" ] && [ "$expected" != "$actual" ]; then
      err "checksum mismatch (expected=${expected} got=${actual})"
    fi
    [ -n "$actual" ] && log "checksum verified"
  else
    log "warning: ${asset} not listed in checksums.txt — skipping verification"
  fi
else
  log "warning: could not fetch checksums.txt — skipping verification"
fi

# ---------------------------------------------------------------------------
# Extract and install
# ---------------------------------------------------------------------------
tar -xzf "$tmp/$asset" -C "$tmp"
[ -f "$tmp/$BINARY" ] || err "archive did not contain a '$BINARY' binary"

mkdir -p "$INSTALL_DIR"
mv "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

log "installed $BINARY $VERSION to $INSTALL_DIR/$BINARY"

# ---------------------------------------------------------------------------
# PATH hint
# ---------------------------------------------------------------------------
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    cat <<EOF

note: $INSTALL_DIR is not on your PATH.
  add this to your shell profile (~/.zshrc, ~/.bashrc, etc.):

    export PATH="$INSTALL_DIR:\$PATH"

EOF
    ;;
esac
