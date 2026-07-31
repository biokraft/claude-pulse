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

# Unit tests
"$SDK/monkeyc" --unit-test -f watch/monkey.jungle -d fr57047mm -o /tmp/t.prg -y developer_key.der
"$SDK/monkeydo" /tmp/t.prg fr57047mm -t

# Store package (multi-device; the store rejects .prg)
"$SDK/monkeyc" -e -f watch/monkey.jungle -o ~/Desktop/ClaudePulse.iq -y developer_key.der -r
```

`fr57047mm` is the device to develop against — it is the watch actually owned.

## Design workflow

The mockup in `design_handoff_garmin_claude_widget/Garmin Watch Claude Usage Tracker/Garmin Widget.dc.html`
is the source of truth for layout. It renders at a **400 px reference screen**, so every
Monkey C view scales from `scale = dc.getWidth() / 400.0` and uses the mockup's own
numbers (ring outer 104, sprite box 52, row block 260, gaps 8/12/14 …).

Order of work when the design is off:

1. Read the mockup HTML and write the measurements down in
   `docs/superpowers/specs/` — one spec file per design pass. Do not eyeball from
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
  `design_handoff_garmin_claude_widget/Garmin Watch Claude Usage Tracker/store-assets/`.
- **`ClaudePulse.iq`** — copied from the Desktop if the export is there.

`FORM-ANSWERS.md` in that folder holds the copy for every field on the submission form;
it is written by hand, not generated, so carry it over when regenerating.

All store images have size limits: screen images 150 KB, cover 300 KB, hero 2048 KB. The
script prints each file's size at the end — check that column before uploading.

## Fake data

The simulator has no relay, so `ClaudePulseApp.seedFakeDataForScreenshots()` writes a
snapshot via `Snap.save(...)` matching the mockup's values (68 % / 42 %, 2 jobs,
$14.82, 2.1M tokens). It is a temporary dev aid: **remove the function and its call in
`onStart()` before any real build or store export.**

## Store

Submission needs the `.iq` export, the store assets under
`design_handoff_garmin_claude_widget/Garmin Watch Claude Usage Tracker/store-assets/`,
and `watch/manifest.xml`'s device list (current-gen smartwatches only — no Edge,
handheld, or aviation products). Anthropic has not yet been asked for permission to use
the Claude name and mascot; that is still open before the listing goes public.
