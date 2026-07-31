#!/usr/bin/env bash
# Build + screenshot each app page from the Connect IQ simulator, headlessly.
#
# The simulator has no scriptable input (osascript needs assistive access), so
# paging is done by temporarily rewriting getInitialView() to open the page we
# want, then capturing the simulator window by its CoreGraphics window id.
#
# Requires a python with pyobjc-framework-Quartz on PYTHON (see scripts/sim-window-id.py).
#
# Usage: scripts/shoot-pages.sh [outdir]
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$REPO/.screenshots}"
DEVICE="fr57047mm"
APP="$REPO/watch/source/ClaudePulseApp.mc"
SDK="$(ls -d "$HOME/Library/Application Support/Garmin/ConnectIQ/Sdks"/*/bin | tail -1)"
PYTHON="${PYTHON:-python3}"
export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk}"

mkdir -p "$OUT"
cp "$APP" "$APP.bak"
trap 'mv -f "$APP.bak" "$APP"' EXIT

shoot() { # shoot <view-class> <page-name>
  perl -0pi -e "s/return \[new \w+View\(\), new PageDelegate\(\d\)\]/return [new $1(), new PageDelegate(0)]/" "$APP"
  # Without a glance entry point the simulator has to open the app view.
  perl -0pi -e "s/function getGlanceView\(/function getGlanceViewDisabled(/" "$APP"
  "$SDK/monkeyc" -f "$REPO/watch/monkey.jungle" -d "$DEVICE" \
    -o /tmp/ClaudePulse-shot.prg -y "$REPO/developer_key.der" >/dev/null
  pkill -f monkeydo 2>/dev/null || true
  sleep 3
  # A simulator left in glance-preview mode keeps showing the glance strip for
  # every later launch, so start from a fresh simulator process each time.
  pkill -f 'ConnectIQ.app/Contents/MacOS' 2>/dev/null || true
  sleep 2
  ("$SDK/connectiq" >/tmp/connectiq-shot.log 2>&1 &)
  sleep 25
  ("$SDK/monkeydo" /tmp/ClaudePulse-shot.prg "$DEVICE" >/tmp/monkeydo-shot.log 2>&1 &)
  sleep 15
  screencapture -o -x -l "$("$PYTHON" "$REPO/scripts/sim-window-id.py")" "$OUT/$2.png"
  echo "wrote $OUT/$2.png"
}

shoot RingsView  page-1-rings
shoot DetailView page-2-detail
shoot CostView   page-3-cost
rm -f "$OUT/_full.png"
