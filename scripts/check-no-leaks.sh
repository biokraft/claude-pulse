#!/usr/bin/env bash
#
# Scans every tracked file for personal data that must never reach the public
# repo: real tunnel hostnames, home paths, and stray hex ids. A leak here is
# unrecoverable once pushed — git history is public forever — so this runs in
# CI on every push, not just as a manual habit.
#
# Scans `git ls-files` (tracked files only), not the working tree, so
# untracked scratch files never trip it and staged-but-uncommitted leaks are
# still caught before they land in history.

set -euo pipefail

fail=0

report() {
  # $1: rg output (file:line:match), $2: explanation
  local hits="$1" reason="$2"
  if [ -n "$hits" ]; then
    fail=1
    while IFS= read -r line; do
      printf '%s  <- %s\n' "$line" "$reason"
    done <<<"$hits"
  fi
}

# --- trycloudflare.com hostnames -------------------------------------------
hits="$(git ls-files -z \
  | xargs -0 rg -n '[a-z0-9-]+\.trycloudflare\.com' --no-heading -- \
  | rg -v '(^|[^.[:alnum:]-])(x|tall-cactus-abc123)\.trycloudflare\.com' \
  | rg -v '<your-url>\.trycloudflare\.com' \
  || true)"
report "$hits" "looks like a real Cloudflare quick-tunnel hostname"

# --- tailscale .ts.net hostnames --------------------------------------------
hits="$(git ls-files -z \
  | xargs -0 rg -n '[a-z0-9-]+\.ts\.net' --no-heading -- \
  | rg -v '<your-' \
  || true)"
report "$hits" "looks like a real Tailscale hostname"

# --- real home paths ---------------------------------------------------------
# /Users/x is the placeholder username used by relay/internal/service's own
# test fixtures, and /home/linuxbrew is Homebrew's fixed install path (not a
# per-user home) mentioned in a comment there — neither names a real person.
hits="$(git ls-files -z \
  | xargs -0 rg -n '(/Users/[a-z]|/home/[a-z])' --no-heading -- \
  | rg -v '/Users/x([^a-zA-Z]|$)' \
  | rg -v '/home/linuxbrew' \
  || true)"
report "$hits" "looks like a real home directory path (use \$HOME or ~ instead)"

# --- stray 32-hex-char ids (excluding the Connect IQ app id) ----------------
# The app id is read from the manifest rather than written here: it is public
# by definition (every store URL contains it), and hardcoding it means a
# rotated id fails this check as a leak while the stale one stays allowed.
app_id="$(sed -n 's/.*<iq:application[^>]*id="\([a-f0-9]\{32\}\)".*/\1/p' \
  "$(git rev-parse --show-toplevel)/watch/manifest.xml" | head -1)"
[ -n "$app_id" ] || { echo "could not read the app id from watch/manifest.xml" >&2; exit 1; }

# go.sum (long hex-ish hashes) and *.png (binary) are excluded via pathspec,
# not piped through another rg -z stage, so the null-separated file list from
# git ls-files never gets re-split and handed to xargs mangled.
hits="$(git ls-files -z -- . ':!go.sum' ':!*.png' \
  | xargs -0 rg -n '\b[a-f0-9]{32}\b' --no-heading -- \
  | rg -v "$app_id" \
  || true)"
report "$hits" "looks like a stray 32-char hex id (secret/token/hash?)"

# --- this machine's own identifiers -----------------------------------------
# Derived at runtime, never written down: hardcoding the maintainer's username
# or tailnet as a search pattern would publish the very strings this script
# exists to keep out. On CI these resolve to the runner's throwaway values and
# match nothing, which is fine — the check that matters runs before a push.
self_patterns=()
if command -v whoami >/dev/null 2>&1; then
  user="$(whoami)"
  # Skip names too generic to be identifying, or we would flag ordinary words.
  case "$user" in
    root|runner|user|admin|build|ubuntu|"") ;;
    *) [ "${#user}" -ge 4 ] && self_patterns+=("$user") ;;
  esac
fi
if command -v tailscale >/dev/null 2>&1; then
  # Only the machine's short hostname. Deliberately NOT the tailnet name: a
  # tailnet is often named after the GitHub org (here it is), which is public
  # by definition and appears in every import path and URL in the tree. Full
  # tailnet hostnames are already covered by the .ts.net rule above.
  while IFS= read -r name; do
    [ -n "$name" ] && self_patterns+=("$name")
  done < <(tailscale status --json 2>/dev/null \
    | jq -r '.Self.DNSName // empty | split(".")[0] | select(length >= 4)' 2>/dev/null || true)
fi

# Anything already in the remote URL or the Go module path is public by
# construction — the repo owner and name cannot leak into their own repo.
public_ctx="$(git remote get-url origin 2>/dev/null || true) $(head -1 relay/go.mod 2>/dev/null || true)"

for pat in "${self_patterns[@]:-}"; do
  [ -z "$pat" ] && continue
  if [ -n "$public_ctx" ] && printf '%s' "$public_ctx" | rg -qi -F "$pat"; then
    continue
  fi
  hits="$(git ls-files -z -- . ':!*.png' \
    | xargs -0 rg -n -F -i "$pat" --no-heading -- || true)"
  report "$hits" "contains an identifier from this machine (username/host/tailnet)"
done

if [ "$fail" -ne 0 ]; then
  echo
  echo "check-no-leaks: found personal data in tracked files (see above)." >&2
  exit 1
fi

echo "check-no-leaks: no personal data found in tracked files."
