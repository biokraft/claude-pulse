# Design

The visual source of truth for the watch app.

| File | What it is |
| --- | --- |
| `mockup.dc.html` | The interactive mockup of all three pages. Open it in a browser (it needs `support.js` alongside it) and drag, or use the side buttons, to page through. |
| `store-assets.dc.html` | Mockup of the Connect IQ Store listing assets. |
| `sprites/` | The "clawd" mascot poses as SVGs. `watch/resources/` carries the subset the app ships. |
| `store-assets/` | Cover image and the two 128x128 device icons, uploaded to the store as-is. |

## Working from the mockup

`mockup.dc.html` renders at a **400 px reference screen**. Every measurement in
[`../docs/design-spec.md`](../docs/design-spec.md) is expressed in those units, and the
Monkey C views scale from `dc.getWidth() / 400.0`. Read measurements out of the HTML
rather than eyeballing screenshots — the file carries exact pixel values and `oklch()`
colours, which are translated once into hex constants in `watch/source/views/Chrome.mc`.

Two things do not survive the translation to hardware, and the spec accounts for both:
device fonts are considerably larger than the mockup's 11–13 px text (`FONT_XTINY` is as
small as it goes), and the shipped sprite art carries transparent padding, so its box has
to be drawn larger than the mockup's 52 px to look the same size.

The mockup is a design reference, not production code — don't port its HTML, CSS or JS.

## Mascot

The mascot is Anthropic's "clawd" character, used here as a homage in an unofficial,
non-commercial project. It is not covered by this repository's MIT license and is not
Anthropic-endorsed.
