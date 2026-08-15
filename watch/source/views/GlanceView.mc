import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Application.Properties;

// The palette is repeated here rather than taken from Chrome because a glance
// runs in its own, much smaller memory budget: anything reachable from this
// file has to be annotated (:glance) and is loaded with it. Chrome carries the
// full-page drawing code, which a glance must not pay for.
(:glance)
const G_TEXT = 0xF7F5F2;
(:glance)
const G_MUTED = 0xA9A399;
(:glance)
const G_TRACK = 0x3A3733;
(:glance)
const G_STALE = 0x8A857C;

(:glance)
class GlanceView extends WatchUi.GlanceView {
    function initialize() { GlanceView.initialize(); }

    function onUpdate(dc as Graphics.Dc) as Void {
        // No dc.clear(). A glance is drawn onto the carousel's own background,
        // and clearing to black painted an opaque box over it — a hard-edged
        // black band across the widget, with the system's background visible
        // above and below. Transparent backgrounds let the strip show through.
        var w = dc.getWidth();
        var h = dc.getHeight();

        var accent = Properties.getValue("accentColor") as Number;
        var nowEpoch = Time.now().value();
        var stored = Snap.load();

        var five = null;
        var seven = null;
        if (stored != null) {
            var d = stored["d"] as Dictionary;
            if (d != null) {
                five = d["five_hour_pct"];
                seven = d["seven_day_pct"];
            }
        }
        var stale = (stored != null) && Snap.isStale(stored, nowEpoch);

        // The title row carries the name and, when the data is old, its age —
        // a glance that silently shows figures from yesterday is worse than
        // one that admits it.
        dc.setColor(stale ? G_STALE : G_MUTED, Graphics.COLOR_TRANSPARENT);
        dc.drawText(0, h / 2, Graphics.FONT_GLANCE, "CLAUDE",
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);

        var titleW = dc.getTextWidthInPixels("CLAUDE", Graphics.FONT_GLANCE);
        var x = titleW + (w * 0.06).toNumber();

        if (stale) {
            dc.setColor(G_STALE, Graphics.COLOR_TRANSPARENT);
            dc.drawText(x, h / 2, Graphics.FONT_GLANCE,
                Snap.syncedLabel(Snap.ageMinutes(stored, nowEpoch)),
                Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
            return;
        }

        // Two meters, each a track with a filled portion and its percentage.
        // Bars beat the old "73% · 23%" because the glance is read at arm's
        // length in a scrolling list: the fill is legible before the digits are.
        var avail = w - x;
        var cellW = (avail / 2).toNumber();
        drawMeter(dc, x, h, cellW, five, accent);
        drawMeter(dc, x + cellW, h, cellW, seven, accent);
    }

    // drawMeter renders one quota as a percentage above its own track. pct is
    // null before the first successful fetch, which draws an empty track rather
    // than a misleading zero-length fill.
    private function drawMeter(dc as Graphics.Dc, x as Number, h as Number,
                               cellW as Number, pct, accent as Number) as Void {
        var barW = (cellW * 0.72).toNumber();
        var barH = (h * 0.14).toNumber();
        if (barH < 3) { barH = 3; }
        var r = barH / 2;

        var label = (pct != null) ? (pct as Number).format("%d") + "%" : "--";
        var color = (pct != null) ? Snap.pctColor(pct as Number, accent) : G_MUTED;

        dc.setColor(pct != null ? G_TEXT : G_MUTED, Graphics.COLOR_TRANSPARENT);
        dc.drawText(x, h * 0.34, Graphics.FONT_GLANCE, label,
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);

        var barY = (h * 0.66).toNumber();
        dc.setColor(G_TRACK, Graphics.COLOR_TRANSPARENT);
        dc.fillRoundedRectangle(x, barY, barW, barH, r);

        if (pct != null) {
            var p = pct as Number;
            if (p > 100) { p = 100; }
            if (p < 0) { p = 0; }
            var fillW = (barW * p / 100.0).toNumber();
            // A non-zero quota always shows something: a sliver the eye can
            // find beats a bar that looks identical to empty.
            if (fillW < barH && p > 0) { fillW = barH; }
            if (fillW > 0) {
                dc.setColor(color, Graphics.COLOR_TRANSPARENT);
                dc.fillRoundedRectangle(x, barY, fillW, barH, r);
            }
        }
    }
}
