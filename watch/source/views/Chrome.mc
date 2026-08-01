import Toybox.Graphics;
import Toybox.Lang;

// Shared palette and chrome drawn on every page, mapped from the design
// handoff mockup (docs/superpowers/specs/2026-07-31-widget-design-conformance.md).
module Chrome {
    const TEXT_PRIMARY = 0xF7F5F2;
    const MUTED = 0xA9A399;
    const DIM = 0x8A857C;
    const TRACK = 0x232323;
    const BAR_IDLE = 0x2E2E2E;
    const DOT_IDLE = 0x404040;

    const PAGE_COUNT = 3;

    // 3-dot carousel indicator: diameter 6, gap 7, 20px above the bottom edge
    // at the 400px reference size.
    function drawPageDots(dc as Graphics.Dc, page as Number, accent as Number) as Void {
        var w = dc.getWidth();
        var h = dc.getHeight();
        var scale = w / 400.0;

        var r = (3 * scale).toNumber();
        if (r < 2) { r = 2; }
        var gap = (7 * scale).toNumber();
        if (gap < 2) { gap = 2; }

        var step = r * 2 + gap;
        var totalW = PAGE_COUNT * r * 2 + (PAGE_COUNT - 1) * gap;
        var x = w / 2 - totalW / 2 + r;
        var y = h - (20 * scale).toNumber() - r;

        for (var i = 0; i < PAGE_COUNT; i += 1) {
            dc.setColor(i == page ? accent : DOT_IDLE, Graphics.COLOR_BLACK);
            dc.fillCircle(x + i * step, y, r);
        }
    }
}
