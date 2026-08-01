#!/usr/bin/env bash
#
# Claude Pulse relay installer.
#
#   curl -fsSL https://raw.githubusercontent.com/biokraft/claude-pulse/main/install.sh | bash
#
# Builds claude-pulse-relay from source and installs it into a bin directory on
# your PATH. Works both piped from curl (clones into a temp dir) and run from a
# checkout (builds the checkout you already have).
#
# Environment overrides:
#   PREFIX   install directory        (default: ~/.local/bin, or /usr/local/bin if already on PATH)
#   REF      git ref to build         (default: main)
#   REPO     git URL to clone         (default: https://github.com/biokraft/claude-pulse.git)

set -euo pipefail

REPO="${REPO:-https://github.com/biokraft/claude-pulse.git}"
REF="${REF:-main}"
GO_MIN_MAJOR=1
GO_MIN_MINOR=25

# ---------------------------------------------------------------- output ----

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; CYAN=$'\033[36m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; CYAN=""; RESET=""
fi

step()  { printf '%s==>%s %s%s\n' "$CYAN$BOLD" "$RESET" "$BOLD" "$1$RESET"; }
info()  { printf '    %s\n' "$1"; }
warn()  { printf '%s!%s %s\n' "$YELLOW$BOLD" "$RESET" "$1" >&2; }
ok()    { printf '%s✓%s %s\n' "$GREEN" "$RESET" "$1"; }
die()   { printf '%serror:%s %s\n' "$RED$BOLD" "$RESET" "$1" >&2; exit 1; }

on_path() { case ":$PATH:" in *":$1:"*) return 0 ;; *) return 1 ;; esac; }

# --------------------------------------------------------- preflight ----

case "$(uname -s)" in
  Darwin|Linux) ;;
  *) die "unsupported platform: $(uname -s). The relay runs on macOS and Linux." ;;
esac

command -v git >/dev/null 2>&1 || die "git is required but not installed."

if ! command -v go >/dev/null 2>&1; then
  die "Go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ is required but not installed.
       macOS: brew install go
       Linux: https://go.dev/doc/install"
fi

go_version="$(go env GOVERSION 2>/dev/null || echo unknown)"
gv="${go_version#go}"
gv_major="${gv%%.*}"
gv_rest="${gv#*.}"
gv_minor="${gv_rest%%.*}"
if [ "${gv_major:-0}" -lt "$GO_MIN_MAJOR" ] ||
   { [ "${gv_major:-0}" -eq "$GO_MIN_MAJOR" ] && [ "${gv_minor:-0}" -lt "$GO_MIN_MINOR" ]; }; then
  die "Go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ is required, found ${go_version}."
fi

# Pick an install directory. Prefer one already on PATH so the binary is
# runnable the moment this script finishes.
if [ -n "${PREFIX:-}" ]; then
  bindir="$PREFIX"
elif on_path "$HOME/.local/bin"; then
  bindir="$HOME/.local/bin"
elif [ -w /usr/local/bin ] && on_path /usr/local/bin; then
  bindir="/usr/local/bin"
else
  bindir="$HOME/.local/bin"
fi

# ------------------------------------------------------------ sources ----

# When run from a checkout, build that checkout: it is what the user is looking
# at, and it avoids a pointless network round trip.
script_dir=""
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

cleanup_dir=""
# Must not end on a failed test: the trap's status becomes the script's.
cleanup() { if [ -n "$cleanup_dir" ]; then rm -rf "$cleanup_dir"; fi; }
trap cleanup EXIT

if [ -n "$script_dir" ] && [ -f "$script_dir/relay/go.mod" ]; then
  src="$script_dir"
  step "Building from this checkout"
  info "$src"
else
  src="$(mktemp -d)"
  cleanup_dir="$src"
  step "Fetching claude-pulse ($REF)"
  git clone --depth 1 --branch "$REF" --quiet "$REPO" "$src" \
    || die "failed to clone $REPO at $REF"
fi

# -------------------------------------------------------------- build ----

step "Building claude-pulse-relay"
mkdir -p "$bindir"
target="$bindir/claude-pulse-relay"
build_version="$(cd "$src" && git describe --tags --always 2>/dev/null || echo "$REF")"
( cd "$src/relay" && go build -trimpath \
    -ldflags "-X main.version=$build_version" \
    -o "$target" ./cmd/claude-pulse-relay ) \
  || die "build failed"
ok "installed $target"

# ------------------------------------------------------ post-install ----

echo
if ! command -v claude-pulse-relay >/dev/null 2>&1; then
  warn "$bindir is not on your PATH. Add it:"
  info ""
  info "    echo 'export PATH=\"$bindir:\$PATH\"' >> ~/.zshrc && exec zsh"
  info ""
fi

if ! command -v cloudflared >/dev/null 2>&1; then
  warn "cloudflared not found — needed for the built-in tunnel."
  if [ "$(uname -s)" = "Darwin" ]; then
    info "    brew install cloudflared"
  else
    info "    https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
  fi
  info "    (or run the relay with --no-tunnel and supply your own HTTPS route)"
  echo
fi

cat <<EOF
${BOLD}Next steps${RESET}

  ${CYAN}1.${RESET} Start the relay — prints your pairing URL, token and a QR code:

       ${BOLD}claude-pulse-relay${RESET}

  ${CYAN}2.${RESET} On your phone, open ${BOLD}Garmin Connect → Connect IQ apps → Claude Pulse →
     Settings${RESET} and enter that URL and token.

  ${CYAN}3.${RESET} Optional — keep it running across reboots, and feed the cost page:

       ${BOLD}claude-pulse-relay service install${RESET}   ${DIM}# launchd / systemd user service${RESET}
       ${BOLD}claude-pulse-relay hook install${RESET}      ${DIM}# Claude Code statusline hook${RESET}

  ${DIM}claude-pulse-relay help${RESET} lists every command.
EOF
