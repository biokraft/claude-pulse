# Claude Pulse — working notes

Garmin Connect IQ watch app (`watch/`) showing Claude Code usage, fed by a self-hosted
Go relay (`relay/`). MIT licensed, free, not monetized.

## Toolchain

Every Connect IQ SDK tool needs `JAVA_HOME=/opt/homebrew/opt/openjdk`. The SDK bin
directory is the newest entry under
`~/Library/Application Support/Garmin/ConnectIQ/Sdks/*/bin`.

```bash
export JAVA_HOME=/opt/homebrew/opt/openjdk
SDK="$(ls -d "$HOME/Library/Application Support/Garmin/ConnectIQ/Sdks"/*/bin | tail -1)"

# Dev build for one device (sideload / simulator)
"$SDK/monkeyc" -f watch/monkey.jungle -d fr57047mm -o /tmp/ClaudePulse.prg -y developer_key.der

# Sideloadable dev build under its own app id, installs next to the store app
scripts/build-dev.sh fr57047mm

# Unit tests
"$SDK/monkeyc" --unit-test -f watch/monkey.jungle -d fr57047mm -o /tmp/t.prg -y developer_key.der
"$SDK/monkeydo" /tmp/t.prg fr57047mm -t

# Store package (multi-device; the store rejects .prg)
"$SDK/monkeyc" -e -f watch/monkey.jungle -o ~/Desktop/ClaudePulse.iq -y developer_key.der -r
```

`fr57047mm` is the device to develop against — it is the watch actually owned.

## Development tunnel

The relay's built-in tunnel (`cloudflared` quick tunnel) is fine for real use but
painful for development: its URL rotates on every restart, forcing a re-pair, and it
can die silently — a `cloudflared` process exiting while the relay keeps running,
leaving the watch stuck on a dead URL with nothing in the log to explain why.

For development, front the relay with a Tailscale Funnel instead. It gives a
permanent HTTPS URL at `https://<your-machine>.<your-tailnet>.ts.net`, so the watch is
paired once and stays paired across restarts.

```bash
# One-time: your tailnet's ACL needs Funnel enabled for this device
#   nodeAttrs: ["funnel"]   — set in the admin console

# Start the relay locally (default port 8787), then front it:
scripts/dev-tunnel.sh

# Stop the Funnel when done:
tailscale funnel --https=443 off
```

To check the whole chain — relay, that Funnel, the usage poll — in one shot:

```bash
claude-pulse-relay status -url "https://$(tailscale status --json | jq -r .Self.DNSName | sed 's/\.$//')"
```

`-url` is needed because with `no_tunnel` set the relay starts no tunnel of its own,
so there is no recorded URL for `status` to check.

`scripts/dev-tunnel.sh` derives the hostname from `tailscale status --json` at
runtime and prints the pairing URL and token — it never hardcodes a hostname,
since this repo is public.

If a relay service is installed (`~/Library/LaunchAgents/com.claudepulse.relay.plist`
or `~/.config/systemd/user/claude-pulse-relay.service`), it opens its own quick tunnel
and competes with the Funnel. A service runs with no arguments, so `--no-tunnel` cannot
reach it; set it in the config instead:

```bash
jq '.no_tunnel = true' ~/.claude-pulse/config.json > /tmp/c && mv /tmp/c ~/.claude-pulse/config.json
claude-pulse-relay service install    # reload
```

That file lives outside the repository, so nothing machine-specific is ever committed.

## Design workflow

The mockup in `design/mockup.dc.html`
is the source of truth for layout. It renders at a **400 px reference screen**, so every
Monkey C view scales from `scale = dc.getWidth() / 400.0` and uses the mockup's own
numbers (ring outer 104, sprite box 52, row block 260, gaps 8/12/14 …).

Order of work when the design is off:

1. Read the mockup HTML and write the measurements down in
   `docs/design-spec.md`. Do not eyeball from
   screenshots; the HTML carries the exact px values and colours.
2. Convert `oklch(...)` colours once into hex constants in `watch/source/views/Chrome.mc`
   and reuse them. Never re-derive palette values per view.
3. Implement, run the unit tests, then screenshot and compare against the mockup.

Known translation gaps to expect: device fonts are much larger than the mockup's
11–13 px text (`FONT_XTINY` is the floor), and the shipped mascot art has transparent
padding, so its box must be larger than the mockup's 52 px to look the same.

## Screenshots

`scripts/shoot-pages.sh [outdir]` builds and captures all three pages. It exists because
the simulator resists automation, and each workaround in it was needed:

- **No input automation.** `osascript` needs assistive access, which is not granted, and
  `cliclick` is not installed — so pages cannot be reached by pressing buttons. The script
  temporarily rewrites `getInitialView()` to open the page it wants, and restores the file
  on exit via a trap.
- **The simulator opens the glance strip, not the app.** A build with a `getGlanceView()`
  entry point always lands on the glance. The script renames that function for the
  screenshot build only.
- **A simulator left in glance mode stays there**, so the script kills and relaunches
  `connectiq` before each page and waits ~25 s for it to boot.
- **The window may sit on another display or behind others**, which breaks region capture.
  `scripts/sim-window-id.py` resolves the CoreGraphics window id by title so
  `screencapture -l <id>` grabs it regardless. It needs `pyobjc-framework-Quartz`:
  create a venv, `pip install pyobjc-framework-Quartz`, and pass it as
  `PYTHON=/path/to/venv/bin/python scripts/shoot-pages.sh`.

Playwright cannot help here — it drives browsers, and the simulator is a native app.

## Store assets

`scripts/build-store-assets.py` turns the raw window captures into the whole upload
folder, so a design change only costs two commands:

```bash
VENV=/path/to/venv/bin/python           # the venv with pyobjc + pillow
PYTHON=$VENV scripts/shoot-pages.sh .screenshots
$VENV scripts/build-store-assets.py .screenshots ~/Desktop/ClaudePulse-store
```

What it produces, and how:

- **Screen images** (`screen-1-rings`, `screen-2-detail`, `screen-3-cost`) — the round
  display is cut out of each window capture using the fractions at the top of the
  script (centre `0.5`/`0.492`, side `0.53` of the window width), resized to 416x416 and
  masked to a circle so the simulator bezel never shows. Those fractions are empirical:
  if the crop is too tight the text nearest the display edge gets clipped — page 2's
  `resets in 3d 7h` is the canary. Re-check them if the simulator window size changes.
- **Hero image** (`hero-1440x720.png`) — the three finished screen images drawn as watch
  discs on the `#141312` background with the app name above. Built from the screen
  images, not the raw captures, so the framing always matches the listing.
- **Cover image and both 128x128 icons** — copied unchanged from
  `design/store-assets/`.
- **`ClaudePulse.iq`** — copied from the Desktop if the export is there.

`FORM-ANSWERS.md` in that folder holds the copy for every field on the submission form;
it is written by hand, not generated, so carry it over when regenerating.

All store images have size limits: screen images 150 KB, cover 300 KB, hero 2048 KB. The
script prints each file's size at the end — check that column before uploading.

## Fake data

The simulator has no relay, so every page would render `--`. `scripts/shoot-pages.sh`
generates `watch/source/_ScreenshotSeed.mc` with the mockup's values (68 % / 42 %, 2
jobs, $14.82, 2.1M tokens, a 7-day cost series) and injects a call in `onStart()`, then
deletes both on exit via its trap.

It is generated rather than committed on purpose. The previous version of this was a
`seedFakeDataForScreenshots()` function living in `ClaudePulseApp.mc` with a note here
saying "remember to delete it before a store export" — precisely the kind of thing that
eventually ships fake data to real users. Nothing to remember now: the tree is clean
whether or not the script succeeds. Verify with `git status watch/` after running it.

## Releases

The full runbook is in [RELEASING.md](RELEASING.md); read it before cutting one. The
short version:

- **The relay releases itself.** Land Conventional Commits on `main`, merge the release
  pull request release-please keeps open, and the tag it creates builds and publishes
  the binaries. Never bump a version or edit `CHANGELOG.md` by hand.
- **The watch app does not.** Any change under `watch/` needs `watch/manifest.xml`
  bumped, a `.iq` exported to `build/` (never `dist/` — goreleaser wipes it on every
  run), and a manual upload to Garmin's dashboard. Nothing in CI can do or check this,
  so it is a checklist in RELEASING.md instead.
- **The two carry different version numbers on purpose.** Separate artifacts, separate
  cadences, and the store constrains the watch app's numbering to only ever increase.

## Store

The listing exists as
[Claude Pulse](https://apps.garmin.com/apps/366c5488-cd20-4299-b498-8c5cff30bcba)
(1.1.0, approved by review on 2026-08-19) but is **not currently served**. The store's
API answers that app id with `410 Gone` and
`"The requested app has been removed from the store due to DSA"` — the EU Digital
Services Act trader declaration is outstanding on the developer account, which hides its
apps regardless of review status. An unknown app id answers `404` on the same endpoint,
so the `410` confirms the record is real and deliberately withheld:

```bash
curl -s https://apps.garmin.com/api/appsLibraryExternalServices/api/asw/apps/<app-id>
```

The web page itself is no help here — it renders client-side and returns an empty shell
either way. Use the API when a listing's status is in question.

The record is owned by the developer account that submitted it. Every later release is an
*Upload New Version* against that listing, never a new one — see
[RELEASING.md](RELEASING.md).

A submission needs the `.iq` export, the store assets under
`design/store-assets/`, and `watch/manifest.xml`'s device list (current-gen smartwatches
only — no Edge, handheld, or aviation products).

Anthropic has still not been asked for permission to use the Claude name and mascot.
That was an open question before the listing went public and it remains one now that it
has; nothing about approval by Garmin speaks to it.
