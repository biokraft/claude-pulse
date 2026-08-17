#!/usr/bin/env bash
#
# Prepares a Connect IQ store upload: bumps the manifest, exports the signed
# .iq, and checks the things the store silently rejects hours later.
#
#   scripts/release-watch.sh 1.0.4
#   scripts/release-watch.sh            # re-export the current manifest version
#
# It stops at the browser. Garmin publishes no upload API — see docs/garmin-release.md
# — so the last step is a form, and this script exists to make sure that form is
# the only manual part.

set -euo pipefail

# ---------------------------------------------------------------- output ----

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  case "${COLORTERM:-}" in
    truecolor|24bit)
      CLAY=$'\033[38;2;204;122;86m'; CREAM=$'\033[38;2;247;245;242m'
      MUTED=$'\033[38;2;169;163;153m'; SAGE=$'\033[38;2;111;154;106m'
      RUST=$'\033[38;2;193;90;62m' ;;
    *)
      CLAY=$'\033[38;5;173m'; CREAM=$'\033[38;5;255m'; MUTED=$'\033[38;5;247m'
      SAGE=$'\033[38;5;108m'; RUST=$'\033[38;5;167m' ;;
  esac
  BOLD=$'\033[1m'; RESET=$'\033[0m'
else
  CLAY=""; CREAM=""; MUTED=""; SAGE=""; RUST=""; BOLD=""; RESET=""
fi

step() { printf '%s==>%s %s%s%s\n' "$CLAY$BOLD" "$RESET" "$CREAM$BOLD" "$1" "$RESET"; }
info() { printf '%s    %s%s\n' "$MUTED" "$1" "$RESET"; }
ok()   { printf '%s  ✓ %s%s%s\n' "$SAGE$BOLD" "$RESET$CREAM" "$1" "$RESET"; }
die()  { printf '%s  ✗ %s%s%s\n' "$RUST$BOLD" "$RESET$CREAM" "$1" "$RESET" >&2; exit 1; }

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="$repo/watch/manifest.xml"
outdir="$repo/build"

# --------------------------------------------------------------- checks ----

export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk}"
[ -d "$JAVA_HOME" ] || die "JAVA_HOME is not a directory: $JAVA_HOME"

sdk="$(ls -d "$HOME/Library/Application Support/Garmin/ConnectIQ/Sdks"/*/bin 2>/dev/null | tail -1)"
[ -n "$sdk" ] || die "no Connect IQ SDK found under ~/Library/Application Support/Garmin/ConnectIQ/Sdks"

key="$repo/developer_key.der"
[ -f "$key" ] || die "developer_key.der is missing. Generate one with scripts/gen-dev-key.sh
       — but if this app is already published, you need the ORIGINAL key. The
       store rejects an update signed with a different one."

# ------------------------------------------------------------- version ----

current="$(sed -n 's/.*<iq:application[^>]*version="\([^"]*\)".*/\1/p' "$manifest" | head -1)"
[ -n "$current" ] || die "could not read the version from $manifest"

target="${1:-$current}"
if [ "$target" != "$current" ]; then
  step "Bumping the manifest"
  info "$current -> $target"
  # Only the application element's version attribute; the manifest itself and
  # the XML declaration carry unrelated version attributes.
  perl -0pi -e "s/(<iq:application[^>]*version=\")[^\"]*(\")/\${1}$target\${2}/" "$manifest"
  new="$(sed -n 's/.*<iq:application[^>]*version="\([^"]*\)".*/\1/p' "$manifest" | head -1)"
  [ "$new" = "$target" ] || die "the bump did not take: manifest still reads $new"
  ok "manifest now $target"
else
  step "Exporting the current manifest version"
  info "$target"
fi

appid="$(sed -n 's/.*<iq:application[^>]*id="\([^"]*\)".*/\1/p' "$manifest" | head -1)"

# --------------------------------------------------------------- export ----

step "Exporting the store package"
mkdir -p "$outdir"
out="$outdir/ClaudePulse-$target.iq"
# Never write to dist/: goreleaser owns it and deletes it on every run.
"$sdk/monkeyc" -e -f "$repo/watch/monkey.jungle" -o "$out" -y "$key" -r 2>&1 \
  | tail -3

[ -f "$out" ] || die "no package was produced at $out"
size="$(wc -c < "$out" | tr -d ' ')"
ok "$(basename "$out") ($((size / 1024)) KB)"
# The device count is whatever monkeyc printed above. It is deliberately not
# recomputed here: the manifest lists 57 products that expand to 70 devices,
# so counting the manifest would contradict the build on every run.

# ------------------------------------------------------------ checklist ----

echo
step "Upload it"
info "https://apps.garmin.com/developer/dashboard"
echo
printf '  %s%s\n' "$CREAM" "1. Open the EXISTING Claude Pulse app, then 'Upload New Version'.$RESET"
printf '  %s%s\n' "$MUTED" "   Not 'Add Beta App' — a beta upload is testable only by you, and"
printf '  %s%s\n' "$MUTED" "   Garmin then requires a different appID to publish it.$RESET"
printf '  %s%s\n' "$CREAM" "2. Version: $target$RESET"
printf '  %s%s\n' "$MUTED" "   The store rejects a version it has already seen. This only goes up.$RESET"
printf '  %s%s\n' "$CREAM" "3. Paste the What's New text, then submit.$RESET"
echo
info "appID in this package: $appid"
info "it must match the published listing, or the update is rejected"
echo

if [ "${OPEN:-1}" = "1" ] && command -v open >/dev/null 2>&1; then
  open -R "$out"
fi
