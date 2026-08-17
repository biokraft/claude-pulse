#!/usr/bin/env bash
#
# Prepares a Connect IQ store upload: bumps the manifest, exports the signed
# .iq, and checks the things the store silently rejects hours later.
#
#   scripts/release-watch.sh 1.0.4
#   scripts/release-watch.sh            # re-export the current manifest version
#   scripts/release-watch.sh --beta 1.0.4   # a beta build, under its own app id
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

beta=0
if [ "${1:-}" = "--beta" ]; then
  beta=1
  shift
fi

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

# A beta build must not leave anything behind, including a version bump: the
# snapshot is taken before the first edit rather than between them. Found by
# interrupting a beta run, which restored the app id but kept the new version.
manifest_backup=""
restore_manifest() {
  if [ -n "${manifest_backup:-}" ] && [ -f "$manifest_backup" ]; then
    mv -f "$manifest_backup" "$manifest"
  fi
}
# INT and TERM as well as EXIT: bash does not run an EXIT trap when it is
# killed by an untrapped signal, so Ctrl-C during the ~2 minute export would
# otherwise leave the beta app id and version in the working tree.
trap restore_manifest EXIT INT TERM HUP
if [ "$beta" = "1" ]; then
  manifest_backup="$(mktemp)"
  cp "$manifest" "$manifest_backup"
fi

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

prod_appid="$(sed -n 's/.*<iq:application[^>]*id="\([^"]*\)".*/\1/p' "$manifest" | head -1)"
appid="$prod_appid"

# A beta upload is a separate store record and must carry its own app id. Using
# the production id would bind that id to the beta record, and Garmin documents
# no way back from that — the reported cases end at "contact support".
#
# The id is swapped in only for the export and restored by the trap, so the
# production id cannot be left in the working tree by a failed or interrupted
# run. It lives in an untracked file: the repository is public, and a second
# 32-character hex id in a tracked file is exactly what check-no-leaks.sh is
# there to catch.
beta_id_file="$repo/watch/.beta-app-id"

if [ "$beta" = "1" ]; then
  if [ ! -f "$beta_id_file" ]; then
    command -v uuidgen >/dev/null 2>&1 || die "uuidgen is needed to create a beta app id"
    uuidgen | tr -d - | tr "A-Z" "a-z" > "$beta_id_file"
    info "generated a beta app id at watch/.beta-app-id (untracked, keep it)"
  fi
  appid="$(tr -d "[:space:]" < "$beta_id_file")"
  [ "${#appid}" = "32" ] || die "the beta app id in $beta_id_file is not 32 hex characters"
  [ "$appid" != "$prod_appid" ] || die "the beta app id is identical to the production one"

  perl -0pi -e "s/(<iq:application[^>]*id=\")[^\"]*(\")/\${1}$appid\${2}/" "$manifest"
  step "Building as a BETA app"
  info "app id swapped for this export only; the manifest is restored on exit"
fi

# --------------------------------------------------------------- export ----

step "Exporting the store package"
mkdir -p "$outdir"
if [ "$beta" = "1" ]; then
  out="$outdir/ClaudePulse-$target-beta.iq"
else
  out="$outdir/ClaudePulse-$target.iq"
fi
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
if [ "$beta" = "1" ]; then
  printf '  %s%s\n' "$CREAM" "1. Upload this under Beta Apps, with the 'Beta App' box ticked.$RESET"
  printf '  %s%s\n' "$MUTED" "   Only your account can install it, and it is not reviewed.$RESET"
else
  printf '  %s%s\n' "$CREAM" "1. Open the EXISTING Claude Pulse app, then 'Upload New Version'.$RESET"
  printf '  %s%s\n' "$MUTED" "   Not 'Add Beta App' — that creates a separate, private record and"
  printf '  %s%s\n' "$MUTED" "   Garmin then requires a different appID to publish it.$RESET"
fi
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
