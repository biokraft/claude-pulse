# Handoff: Claude Usage Tracker — Garmin Watch Widget

## Overview
A Garmin Connect IQ widget showing Claude Code usage at a glance: 5h/7d quota %, quota detail with reset countdowns, an animated "clawd" mascot companion reacting to state, active job count, and today's token cost. Backed by an existing self-hosted scrape pipeline (see "Backend / data source" below) — no Anthropic OAuth exists for this, so data comes from a host-side poller the user runs themselves.

## About the design files
The bundled file (`Garmin Widget.dc.html`) is an **HTML/React design reference**, not production code. It was built to preview layout, motion, and interaction in a browser. The real target is **Garmin Connect IQ**, built in **Monkey C** against the Connect IQ SDK (`Toybox.WatchUi`, `Toybox.Graphics`, `Toybox.Timer`, `Toybox.Communications` for phone/API calls). Do not port the HTML/CSS/JS directly — recreate the same visual design, layout proportions, and state logic using native Connect IQ drawables (`WatchUi.View` + `onUpdate(dc)` custom drawing, or `WatchUi.Drawable` layers), following whatever project structure/patterns already exist in the target Connect IQ project (or set one up fresh if this is the first widget).

## Fidelity
**High-fidelity.** Colors, spacing, typography sizes, ring/bar treatments, and page-to-page swipe behavior are final — recreate them precisely. Sprite assets (GIFs/SVGs) are placeholders for 4 of 5 mascot poses; only `clawd-idle-look.svg` is a real final asset today. Treat the other sprite filenames as a defined content contract to wire up once those assets exist.

## Screens / pages
The widget is a single Connect IQ **widget glance + swipeable pages** (3 pages, horizontal swipe / up-down button navigation), inside the round watch's circular screen (400px diameter reference / adapt to actual device resolution).

### Page 1 — Glance
- Purpose: at-a-glance quota check, the default view when opening the widget.
- Layout: centered column. Header label "CLAUDE USAGE" (11px, uppercase, letter-spacing 2px, muted). Below it, two donut rings side by side (104px outer / 80px inner "hole", gap 28px between rings): "5H" ring and "7D" ring, each with a big percentage number (22px bold) centered inside, and a small label below the ring ("5H" / "7D", 11px bold, muted).
- Ring rendering: CSS `conic-gradient(color 0%..pct%, track pct%..100%)` on the outer circle, with a smaller solid circle on top to create the donut hole. In Monkey C, draw with `dc.drawArc()` (arc from 90° going clockwise for pct/100 of 360°) over a full background arc in the track color, then the center number as text.
- Ring color: accent color (`#CC7A56` default, tweakable) normally; switches to a warning red (`oklch(62% 0.19 25)` ≈ `#C24B3A`) when that ring's % is ≥85.

### Page 2 — Quota Detail (mascot + jobs + reset countdowns)
- Purpose: deeper look at quota state, companion status, and running job count.
- Layout, top to bottom, centered column, gap ~8px:
  1. **Mascot sprite** (52×52px), pose-driven — see "Mascot / companion system" below.
  2. **Job count line**: "{{activeCount}} job(s) running" — 11px bold, colored the same as the mascot's pose color (see below).
  3. **Quota rows** (260px wide column, gap 16px): one row per window (5 hour, 7 day). Each row: small color swatch (14×14px rounded square, same color logic as the ring) + a text column: title/percent line ("5 hour" ... "68%", 13px bold, space-between), a thin progress bar underneath (6px tall track `rgba(255,255,255,.08)`, filled bar rounded 3px in the accent/warning color, width = pct%), and a "resets in Xh Ym" caption (11px, muted) below the bar.
- Content used in mock: 5h resets "2h 14m" / 7d resets "4d 6h" normally; "38m" / "1d 3h" in the near-limit demo state.

### Page 3 — Today's Cost
- Purpose: daily spend at a glance.
- Layout: centered column. Header "TODAY'S COST" (same label style as other headers). Big dollar figure (38px bold, e.g. "$14.82"). Token count caption below (13px, muted, e.g. "2.1M tokens"). Below that, a 7-bar mini bar chart (12px wide bars, up to 56px tall, 6px gap, rounded top corners) showing the trailing 7 days; today's bar is highlighted in the accent color, the other 6 are a dim neutral (`rgba(255,255,255,.18)`).

### Navigation / chrome (applies to all pages)
- Circular black (`oklch(8% 0 0)`) screen inset inside a metal bezel (radial gradient dark gray, or silver in the alternate finish). Pages sit in a horizontal track; swiping/dragging left-right moves between them with an eased slide transition (~350ms). 3 small dots at the bottom of the screen indicate current page and are tappable to jump directly.
- Two physical side buttons on the right (UP → previous page, DOWN → next page) and one on the left (BACK → jump to page 1 / glance). These map to hardware buttons in a real Connect IQ widget (`onPreviousPage`/`onNextPage` or button input handlers), not just touch — Garmin devices vary between touch and 5-button models, support both.

## Mascot / companion system
A pixel-art character ("clawd") whose pose reflects app state, shown on Page 2.

**Pose priority / trigger logic** (evaluate top to bottom, first match wins):
1. **celebrate** — active during a `confettiUntil` time window (e.g. right after a quota reset). Sprites: `clawd-happy.gif`, `clawd-react-double-jump.gif` (pick one at random per trigger).
2. **annoyed** — when 5h or 7d quota % ≥ 85 (mapped from the mock's `cpu`/`mem` ≥85% concept). Sprite: `clawd-react-annoyed.gif`.
3. **working** — when `claudeActive` is true (an active job is running). Sprites (pick random on each transition into this pose): `clawd-typing.gif`, `clawd-thinking.gif`, `clawd-building.gif`, `clawd-carrying.gif`, `clawd-conducting.gif`, `clawd-juggling.gif`, `clawd-sweeping.gif`.
4. **sleeping** — during the night window (00:00–06:00 local time) AND inactive for ≥63s. Sprite: `clawd-sleeping.gif`.
5. **idle** — default fallback (not active, not night, not over quota).
   - Day idle: pick a random variant from `clawd-idle.gif` (default weight), `clawd-headphones-groove.gif`, `clawd-coffee-hand.svg`, `clawd-coffee-head-flip.svg`, `clawd-idle-reading.gif`, `clawd-idle-living.svg`, `clawd-bubble.gif`, `clawd-idle-follow.svg`, `clawd-idle-look.svg` (this last one is the only asset delivered so far — bundled in `sprites/clawd-idle-look.svg`).
   - Night-only idle stage progression (if in idle pose but not yet past the 63s "asleep" threshold), keyed off `inactiveSinceMs`: `clawd-idle-yawn.svg` after 20s inactive → `clawd-idle-doze.svg` after 40s → `clawd-idle-collapse.svg` after 60s.

**Mock/demo behavior in the HTML reference**: a `mascotPose` prop can force any pose for preview (`auto`/`celebrate`/`annoyed`/`working`/`idle`/`sleeping`); `auto` derives the pose from local time + the existing quota/activity demo state. Only the idle pose renders a real image; the other 4 poses render a placeholder (dashed/striped box with the intended filename as a label) — replace those placeholders with the real sprites once available, keeping the same trigger logic.

**Color coupling**: the job-count text and mascot label use the same accent/warning color logic as the quota rings (accent normally, warning red when a quota ring is in its "annoyed" ≥85% state).

## Backend / data source (context for whoever wires up real data)
No Anthropic OAuth exists for third-party "log in with Claude" — this reuses an existing self-hosted scrape pipeline (three independent pollers into SQLite, exposed via a small unauthenticated JSON API over Tailscale):
- `GET /api/widgets/claude-usage` → `{five_hour_pct, seven_day_pct, five_hour_resets_at, seven_day_resets_at, fetched_at}`
- `GET /api/widgets/claude-usage/history?hours=24` → time series
- `GET /api/widgets/claude-activity` → `{is_active, active_count, checked_at}`
- `GET /api/widgets/claude-tokens/daily` → per-day cost/token totals
The underlying `/api/oauth/usage` upstream endpoint 429-rate-limits unpredictably — cache aggressively; poll host-side ≤ every 5 min, and even less often from the watch (1–5 min is fine). The watch cannot reach a LAN endpoint directly — it needs either a public URL or to go through the paired phone (Connect IQ `Communications.makeWebRequest` via the companion phone app). Packaging the fetch script + a minimal always-on HTTP server as a standalone installable (decoupled from the existing Ansible/Tailscale home-infra setup) is an open item, not solved in this design pass.

## Design tokens
- Background (app/bezel surround): `oklch(14% 0.004 60)`
- Screen background (OLED black): `oklch(8% 0 0)`
- Text primary: `oklch(97% 0.004 80)`
- Text muted: `oklch(65% 0.01 80)` / `oklch(55% 0.01 80)` / `oklch(60% 0.01 80)` (varying muted levels used contextually)
- Accent (default): `#CC7A56` — tweakable to `#5B8DEF`, `#6FBF73`, `#B36FD1`
- Warning (≥85% quota): `oklch(62% 0.19 25)` ≈ `#C24B3A`
- Progress track: `rgba(255,255,255,0.08)`
- Bezel (black finish): `radial-gradient(circle at 35% 30%, oklch(34% 0 0), oklch(20% 0 0) 60%, oklch(10% 0 0) 100%)`; silver finish swaps to lighter grays
- Typography: system font stack (`-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif`) — on-device use the Connect IQ system font at equivalent weights/sizes
- Font sizes used: 38px (cost figure), 26px (unused now), 22px (ring %), 18px, 13px, 12px, 11px, 9px
- Corner radii: 3px (progress bars/swatches), 4px (buttons), 12–16px (placeholder mascot box)
- Screen diameter reference: 400px (scale proportionally to actual device resolution, e.g. Fenix/Forerunner round displays)

## Assets
- `sprites/clawd-idle-look.svg` — real, final. Pixel-art mascot, ~45×45 viewBox, single accent-tone fill (`#DE886D`), used for the idle pose.
- All other mascot sprite filenames referenced above (`clawd-happy.gif`, `clawd-react-annoyed.gif`, `clawd-typing.gif`, etc. — 19 files total across poses) are **not yet delivered**; the design currently shows striped placeholders labeled with each filename in their place. Source them and drop in before shipping.

## Files
- `Garmin Widget.dc.html` — the interactive HTML design reference (open in a browser to review layout/motion/interaction).
- `sprites/clawd-idle-look.svg` — the one real mascot asset.
