# Widget Design Conformance Spec

Source of truth: `design/mockup.dc.html`.
All measurements are in the mockup's 400 px reference screen. On device, `scale = dc.getWidth() / 400.0`.

## Global Constraints

- Background: `#141414` (mockup `oklch(8% 0 0)`), drawn as `COLOR_BLACK` clear.
- Text primary: `#F7F5F2` (`oklch(97% 0.004 80)`).
- Text muted (labels, page titles): `#A9A399` (`oklch(65% 0.01 80)`).
- Text dim (captions, idle status): `#8A857C` (`oklch(55% 0.01 80)`).
- Track / inactive fill: `#232323` (`rgba(255,255,255,.08)` over the background).
- Accent: user property `accentColor`, default `0xCC7A56`.
- Warn: `0xC24B3A`, applied when a percentage is >= 85.
- Every page is a vertically centered column, not top-anchored.
- Every page draws the 3-dot page indicator, centered, 20 px above the screen bottom:
  dot diameter 6, gap 7, current page in accent, others `#404040`.

## Page 1 — Rings

Column gap 14, padding 30.

1. Title `CLAUDE USAGE`, uppercase, 11 px, muted.
2. Ring pair, horizontal gap 28. Each ring:
   - outer diameter 104, inner hole diameter 80 → arc radius 46, pen width 12.
   - track full circle in `#232323`; value arc clockwise from 12 o'clock, `pct` of 360.
   - centre text `<pct>%` (percent sign required), 22 px bold, text primary.
   - caption below ring, gap 8: `5H` (left) and `7D` (right), 11 px, muted.
   - ring centres sit at `cx ± 66`.
3. Status row, gap 6 between parts: filled dot diameter 9, then label.
   - active: label `ACTIVE`, dot and label in accent.
   - inactive: label `IDLE`, dot and label dim. The idle state is drawn, never omitted.

## Page 2 — Detail

Column gap 8, padding 22.

1. Mascot sprite, 52 x 52, drawn with `drawScaledBitmap`, pose from `Pose.compute`.
2. `<n> job running` / `<n> jobs running`, 11 px bold, accent (warn colour when pose is `annoyed`).
3. Two quota rows in a 260-wide block, vertical gap 14. Each row:
   - bullet: 14 x 14 rounded square (corner radius 3), colour = that window's pct colour,
     followed by a 14 px gap.
   - the remaining width holds: label left (`5 hour` / `7 day`) and `<pct>%` right, both 13 px,
     text primary; then a 6 px tall bar (radius 3) 6 px below, track `#232323`, fill = pct colour;
     then `resets in <countdown>` 11 px dim, 4 px below the bar.

No pose-name text on this page.

## Page 3 — Cost

Column gap 12, padding 30.

1. Title `TODAY'S COST`, uppercase, 11 px, muted.
2. `$<amount>` with two decimals, 38 px bold, text primary.
3. `<tokens> tokens`, 13 px, muted.
4. 7-bar history chart, 8 px below the caption: bar width 12, gap 6, max height 56,
   3 px top corner radius. The last bar (today) is accent; the rest are `#2E2E2E`.
   No background track behind the bars.

## Stale state

When the snapshot is stale, every value colour degrades to dim and the existing
`synced <n>m ago` caption is drawn above the page dots.
