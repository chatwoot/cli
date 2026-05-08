#!/usr/bin/env sh
# install.sh — download a chatwoot-cli release binary into a local bin dir.
#
# Usage:
#   curl -fsSL https://chwt.app/install-cli | sh
#
# Environment:
#   CHATWOOT_VERSION      pin a specific version, e.g. "v0.2.1" (default: latest)
#   CHATWOOT_INSTALL_DIR  install location (default: $HOME/.local/bin)
#   NO_COLOR              set to disable color/banner output

set -eu

REPO="chatwoot/cli"
BINARY="chatwoot"
INSTALL_DIR="${CHATWOOT_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${CHATWOOT_VERSION:-latest}"

# ---------------------------------------------------------------------------
# Pretty output
# ---------------------------------------------------------------------------
# Colors only when stdout is a TTY and NO_COLOR isn't set.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_RESET=$(printf '\033[0m')
  C_DIM=$(printf '\033[2m')
  C_BOLD=$(printf '\033[1m')
  C_CYAN=$(printf '\033[36m')
  C_GREEN=$(printf '\033[32m')
  C_YELLOW=$(printf '\033[33m')
  C_RED=$(printf '\033[31m')
else
  C_RESET=''; C_DIM=''; C_BOLD=''; C_CYAN=''; C_GREEN=''; C_YELLOW=''; C_RED=''
fi

step() { printf '  %s→%s %s\n' "$C_CYAN" "$C_RESET" "$*"; }
ok()   { printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '  %s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*"; }
err()  { printf '  %s✗%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }

banner() {
  [ -t 1 ] || return 0
  [ -z "${NO_COLOR:-}" ] || return 0
  printf '\n%s' "$C_CYAN"
  cat <<'EOF'
  ┏━┛┃ ┃┏━┃━┏┛┃┃┃┏━┃┏━┃━┏┛  ┏━┛┃  ┛
  ┃  ┏━┃┏━┃ ┃ ┃┃┃┃ ┃┃ ┃ ┃   ┃  ┃  ┃
  ━━┛┛ ┛┛ ┛ ┛ ━━┛━━┛━━┛ ┛   ━━┛━━┛┛
EOF
  printf '%s\n' "$C_RESET"
}

banner

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

step "Detected ${os}/${arch}"

# ---------------------------------------------------------------------------
# Resolve version (latest by default)
# ---------------------------------------------------------------------------
if [ "$VERSION" = "latest" ]; then
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

step "Resolved version ${C_BOLD}${VERSION}${C_RESET}"

# ---------------------------------------------------------------------------
# Build URLs and download
# ---------------------------------------------------------------------------
asset="${BINARY}_${ver_clean}_${os}_${arch}.tar.gz"
asset_url="https://github.com/$REPO/releases/download/${VERSION}/${asset}"
checksum_url="https://github.com/$REPO/releases/download/${VERSION}/checksums.txt"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

step "Downloading ${asset}"
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
      warn "no sha256 tool found — skipping checksum verification"
      actual=""
    fi
    if [ -n "$actual" ] && [ "$expected" != "$actual" ]; then
      err "checksum mismatch (expected=${expected} got=${actual})"
    fi
    [ -n "$actual" ] && step "Verified checksum"
  else
    warn "${asset} not listed in checksums.txt — skipping verification"
  fi
else
  warn "could not fetch checksums.txt — skipping verification"
fi

# ---------------------------------------------------------------------------
# Extract and install
# ---------------------------------------------------------------------------
tar -xzf "$tmp/$asset" -C "$tmp"
[ -f "$tmp/$BINARY" ] || err "archive did not contain a '$BINARY' binary"

mkdir -p "$INSTALL_DIR"
mv "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

ok "Installed ${C_BOLD}${BINARY} ${VERSION}${C_RESET} to ${INSTALL_DIR}/${BINARY}"

# ---------------------------------------------------------------------------
# PATH hint
# ---------------------------------------------------------------------------
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\n'
    warn "${INSTALL_DIR} is not on your PATH"
    printf '    add this to your shell profile (~/.zshrc, ~/.bashrc, etc.):\n\n'
    # shellcheck disable=SC2016  # literal $PATH is intentional here
    printf '      %sexport PATH="%s:$PATH"%s\n\n' "$C_DIM" "$INSTALL_DIR" "$C_RESET"
    ;;
esac

# ---------------------------------------------------------------------------
# Optional: set up tab completion (interactive only)
# ---------------------------------------------------------------------------
# Skip silently when not running on a real terminal (e.g. CI, Dockerfile).
# stdout is checked because `curl ... | sh` keeps it as a TTY but rebinds stdin
# to the curl pipe — we read input from /dev/tty instead.
print_next_steps() {
  printf '\n  %sNext steps%s\n' "$C_BOLD" "$C_RESET"
  printf '    %schatwoot auth login%s        log in to your Chatwoot instance\n' "$C_CYAN" "$C_RESET"
  printf '    %schatwoot --help%s            see all commands\n\n' "$C_CYAN" "$C_RESET"
}

if [ ! -t 1 ] || [ ! -r /dev/tty ]; then
  exit 0
fi

case "${SHELL##*/}" in
  bash) shell_kind=bash ;;
  zsh)  shell_kind=zsh ;;
  fish) shell_kind=fish ;;
  *)
    printf '\n'
    step "Tab completion: run '${C_BOLD}${BINARY} completion --help${C_RESET}' to set up"
    print_next_steps
    exit 0
    ;;
esac

printf '\n  %sSet up tab completion for %s?%s [Y/n] ' "$C_BOLD" "$shell_kind" "$C_RESET"
if ! read -r response < /dev/tty; then
  echo
  print_next_steps
  exit 0
fi
case "$response" in
  n|N|no|NO|No)
    step "Skipped completion setup"
    print_next_steps
    exit 0
    ;;
esac

case "$shell_kind" in
  bash)
    dir="${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions"
    mkdir -p "$dir"
    "$INSTALL_DIR/$BINARY" completion bash -c > "$dir/$BINARY"
    ok "Added bash completion to ${dir}/${BINARY} — restart your shell to enable"
    ;;
  fish)
    dir="${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
    mkdir -p "$dir"
    "$INSTALL_DIR/$BINARY" completion fish -c > "$dir/$BINARY.fish"
    ok "Added fish completion to ${dir}/${BINARY}.fish"
    ;;
  zsh)
    rc="${ZDOTDIR:-$HOME}/.zshrc"
    line="source <($BINARY completion zsh -c)"
    if [ -f "$rc" ] && grep -Fq "$line" "$rc"; then
      step "zsh completion already present in ${rc}"
    else
      printf '\n# chatwoot CLI completion\n%s\n' "$line" >> "$rc"
      ok "Added zsh completion to ${rc} — restart your shell to enable"
    fi
    ;;
esac

print_next_steps
