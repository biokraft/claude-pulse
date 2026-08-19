#!/usr/bin/env bash
#
# Builds a sideloadable development copy of the watch app under its own app id
# and its own name, so it installs alongside the Connect IQ Store build instead
# of replacing it.
#
#   scripts/build-dev.sh                 # build for the default device
#   scripts/build-dev.sh fenix8solar47mm # build for another device
#   INSTALL=1 scripts/build-dev.sh       # also copy it to a mounted watch
#
# The app id is the only thing the watch uses to tell two installs apart. The
# store build carries the production id from watch/manifest.xml; this build
# swaps in an untracked id from watch/.dev-app-id for the duration of the
# compile and restores the manifest afterwards, so the production id can never
# be left out of the working tree by a failed or interrupted run.

set -euo pipefail

DEVICE="${1:-fr57047mm}"

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="$repo/watch/manifest.xml"
strings="$repo/watch/resources/strings/strings.xml"
outdir="$repo/build"

export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk}"
[ -d "$JAVA_HOME" ] || { echo "JAVA_HOME is not a directory: $JAVA_HOME" >&2; exit 1; }

sdk="$(ls -d "$HOME/Library/Application Support/Garmin/ConnectIQ/Sdks"/*/bin 2>/dev/null | tail -1)"
[ -n "$sdk" ] || { echo "no Connect IQ SDK found" >&2; exit 1; }

key="$repo/developer_key.der"
[ -f "$key" ] || { echo "developer_key.der is missing; run scripts/gen-dev-key.sh" >&2; exit 1; }

# Snapshot before the first edit, and trap the signals as well as EXIT: bash
# does not run an EXIT trap when it is killed by an untrapped signal, so a
# Ctrl-C during the compile would otherwise leave the dev id in the tree.
manifest_backup="$(mktemp)"; cp "$manifest" "$manifest_backup"
strings_backup="$(mktemp)";  cp "$strings"  "$strings_backup"
restore() {
  [ -f "$manifest_backup" ] && mv -f "$manifest_backup" "$manifest"
  [ -f "$strings_backup" ]  && mv -f "$strings_backup"  "$strings"
}
trap restore EXIT INT TERM HUP

prod_appid="$(sed -n 's/.*<iq:application[^>]*id="\([^"]*\)".*/\1/p' "$manifest" | head -1)"

# Untracked: the repository is public, and a second 32-character hex id in a
# tracked file is exactly what scripts/check-no-leaks.sh is there to catch.
dev_id_file="$repo/watch/.dev-app-id"
if [ ! -f "$dev_id_file" ]; then
  command -v uuidgen >/dev/null 2>&1 || { echo "uuidgen is needed to create a dev app id" >&2; exit 1; }
  uuidgen | tr -d - | tr "A-Z" "a-z" > "$dev_id_file"
  echo "  generated a dev app id at watch/.dev-app-id (untracked, keep it:"
  echo "  changing it makes the watch treat the next build as a new app)"
fi
appid="$(tr -d "[:space:]" < "$dev_id_file")"
[ "${#appid}" = 32 ] || { echo "the dev app id in $dev_id_file is not 32 hex characters" >&2; exit 1; }
[ "$appid" != "$prod_appid" ] || { echo "the dev app id is identical to the production one" >&2; exit 1; }

perl -0pi -e "s/(<iq:application[^>]*id=\")[^\"]*(\")/\${1}$appid\${2}/" "$manifest"
# Renamed too, or the two installs are indistinguishable in the watch menu.
perl -0pi -e 's{(<string id="AppName">)[^<]*(</string>)}{${1}Pulse Dev${2}}' "$strings"

mkdir -p "$outdir"
out="$outdir/ClaudePulse-dev-$DEVICE.prg"
"$sdk/monkeyc" -f "$repo/watch/monkey.jungle" -d "$DEVICE" -o "$out" -y "$key" 2>&1 | tail -3
[ -f "$out" ] || { echo "no package was produced at $out" >&2; exit 1; }

echo
echo "  built $(basename "$out") as \"Pulse Dev\" (appID $appid)"
echo "  the store build keeps $prod_appid, so both install side by side"

if [ "${INSTALL:-0}" = "1" ]; then
  dest="/Volumes/GARMIN/GARMIN/APPS"
  [ -d "$dest" ] || { echo "no watch mounted at $dest" >&2; exit 1; }
  cp "$out" "$dest/"
  echo "  copied to $dest — eject the watch to finish installing"
fi
