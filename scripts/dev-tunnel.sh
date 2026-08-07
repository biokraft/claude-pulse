#!/usr/bin/env bash
#
# Fronts a locally running claude-pulse-relay with a Tailscale Funnel, so the
# relay gets a stable HTTPS URL for development instead of the quick tunnel
# (cloudflared), whose URL rotates on every restart and can die silently.
#
#   scripts/dev-tunnel.sh
#   PORT=9090 scripts/dev-tunnel.sh
#
# Never hardcodes a hostname: this repo is public, so the hostname is derived
# at runtime from `tailscale status --json` and only ever printed, never
# written down.

set -euo pipefail

PORT="${PORT:-8787}"

# ---------------------------------------------------------------- output ----

# Same Anthropic palette as install.sh (see watch/source/views/Chrome.mc for
# the hex source of truth) so the two scripts look like siblings.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  case "${COLORTERM:-}" in
    truecolor|24bit)
      CLAY=$'\033[38;2;204;122;86m'      # #CC7A56 — accent
      CREAM=$'\033[38;2;247;245;242m'    # #F7F5F2 — primary text
      MUTED=$'\033[38;2;169;163;153m'    # #A9A399 — secondary text
      SAGE=$'\033[38;2;111;154;106m'     # #6F9A6A — success
      RUST=$'\033[38;2;193;90;62m'       # #C15A3E — error
      ;;
    *)
      CLAY=$'\033[38;5;173m'; CREAM=$'\033[38;5;255m'; MUTED=$'\033[38;5;247m'
      SAGE=$'\033[38;5;108m'; RUST=$'\033[38;5;167m'
      ;;
  esac
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
  CLAY=""; CREAM=""; MUTED=""; SAGE=""; RUST=""; BOLD=""; DIM=""; RESET=""
fi

step()  { printf '%s==>%s %s%s%s\n' "$CLAY$BOLD" "$RESET" "$CREAM$BOLD" "$1" "$RESET"; }
info()  { printf '%s    %s%s\n' "$MUTED" "$1" "$RESET"; }
warn()  { printf '%s  ! %s%s%s\n' "$CLAY$BOLD" "$RESET$MUTED" "$1" "$RESET" >&2; }
ok()    { printf '%s  ✓ %s%s%s\n' "$SAGE$BOLD" "$RESET$CREAM" "$1" "$RESET"; }
die()   { printf '%s  ✗ %s%s%s\n' "$RUST$BOLD" "$RESET$CREAM" "$1" "$RESET" >&2; exit 1; }

# --------------------------------------------------------- preflight ----

if ! command -v tailscale >/dev/null 2>&1; then
  die "tailscale is required but not installed.
       macOS: brew install tailscale (or the Mac App Store client)
       Linux: https://tailscale.com/download"
fi

command -v jq >/dev/null 2>&1 || die "jq is required but not installed."

# ------------------------------------------------------------- connect ----

status_json="$(tailscale status --json 2>/dev/null || true)"
backend_state="$(printf '%s' "$status_json" | jq -r '.BackendState // empty')"

if [ "$backend_state" != "Running" ]; then
  step "Tailscale is not running (state: ${backend_state:-unknown}) — connecting"
  tailscale up
  status_json="$(tailscale status --json 2>/dev/null || true)"
fi

host="$(printf '%s' "$status_json" | jq -r '.Self.DNSName // empty' | sed 's/\.$//')"
[ -n "$host" ] || die "could not determine this machine's Tailscale hostname from 'tailscale status --json'."

# --------------------------------------------------------------- funnel ----

step "Starting Funnel on port $PORT"
# The first time Funnel is used on a tailnet it is disabled, and the CLI prints
# an enable link and then BLOCKS, polling until you click it. Without this note
# the script looks frozen.
info "first run on a tailnet? Tailscale prints an enable link and waits here"
info "until you open it — that pause is expected, not a hang."
if ! tailscale funnel --bg "$PORT"; then
  echo
  warn "Funnel failed to start. Your tailnet policy likely needs to allow it:"
  info ""
  info "    Add \"funnel\" to nodeAttrs for this device in the admin console:"
  info "    https://login.tailscale.com/admin/acls"
  info ""
  die "Funnel not started."
fi

ok "Funnel is running"

# ------------------------------------------------------------- service ----

config_file_hint="${CLAUDE_PULSE_HOME:-\$HOME/.claude-pulse}/config.json"
launchd_unit="$HOME/Library/LaunchAgents/com.claudepulse.relay.plist"
systemd_unit="$HOME/.config/systemd/user/claude-pulse-relay.service"
if [ -f "$launchd_unit" ] || [ -f "$systemd_unit" ]; then
  # A service runs with no arguments, so --no-tunnel cannot reach it; the
  # config setting is the only lever.
  warn "an installed relay service was found — it opens its own quick tunnel,"
  warn "which competes with this Funnel. Disable it with:"
  info ""
  info "    jq '.no_tunnel = true' \"$config_file_hint\" > /tmp/c && mv /tmp/c \"$config_file_hint\""
  info "    claude-pulse-relay service install    # reload"
  info ""
  info "That config lives outside this repository, so nothing machine-specific"
  info "is ever committed."
fi

# -------------------------------------------------------------- pairing ----

pulse_home="${CLAUDE_PULSE_HOME:-$HOME/.claude-pulse}"
config_file="$pulse_home/config.json"

token=""
if [ -f "$config_file" ]; then
  token="$(jq -r '.token // empty' "$config_file" 2>/dev/null || true)"
fi

echo
step "Pairing details"
info "URL:   https://$host"
if [ -n "$token" ]; then
  info "Token: $token"
else
  warn "no token found at $config_file (has the relay been started at least once?)"
fi

echo
step "To stop the Funnel"
info "tailscale funnel --https=443 off"
