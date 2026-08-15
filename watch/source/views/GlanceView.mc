import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Application.Properties;
import Toybox.System;

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
        var h = dc.getHeight();
        // dc.getWidth() is the strip's nominal width, not the part of it the
        // user can see: on a round watch the right-hand end passes under the
        // bezel, which clipped the second percentage mid-digit. Rectangular
        // screens lose almost nothing, so the inset is taken from the shape
        // rather than applied blindly everywhere.
        var w = dc.getWidth();
        if (System.getDeviceSettings().screenShape == System.SCREEN_SHAPE_ROUND) {
            // The text row sits nearer the top of the circle than the bar row,
            // so its chord is the shorter of the two. Both rows use it, which
            // keeps the columns aligned and costs the bars a few pixels.
            w = (w * 0.80).toNumber();
        }

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

        // Two rows: the figures with their labels, then their bars. The app
        // name used to occupy the first row, but it plus both percentages
        // never fit — the second figure was clipped mid-digit on a round
        // screen at every font size tried. The launcher icon beside this strip
        // already says which app it is, whereas nothing else says which window
        // each number belongs to, so the space buys "5H" and "7D" instead.
        var rowOne = (h * 0.30).toNumber();
        var rowTwo = (h * 0.74).toNumber();

        var gap = (w * 0.06).toNumber();
        if (gap < 6) { gap = 6; }
        var cellW = ((w - gap) / 2).toNumber();

        if (stale) {
            // A glance that silently shows yesterday's figures is worse than
            // one that admits the data is old, so staleness replaces the
            // meters rather than decorating them.
            dc.setColor(G_STALE, Graphics.COLOR_TRANSPARENT);
            dc.drawText(0, rowOne, Graphics.FONT_GLANCE, "CLAUDE",
                Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
            dc.drawText(0, rowTwo, Graphics.FONT_GLANCE,
                Snap.syncedLabel(Snap.ageMinutes(stored, nowEpoch)),
                Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
            return;
        }

        // FONT_GLANCE is a different width on every device family, so whether
        // "5H 100% 7D 100%" fits cannot be settled by picking numbers here. It
        // is measured, and the percent signs are dropped when it does not —
        // losing a symbol the labels already imply beats clipping a digit.
        var widest = dc.getTextWidthInPixels("5H 100%", Graphics.FONT_GLANCE);
        var suffix = (widest * 2 + gap <= w) ? "%" : "";

        drawQuota(dc, 0, rowOne, rowTwo, cellW, "5H", five, suffix, accent);
        drawQuota(dc, cellW + gap, rowOne, rowTwo, cellW, "7D", seven, suffix, accent);
    }

    // drawQuota renders one window as a labelled figure over its own bar. pct
    // is null before the first successful fetch, which draws an empty track
    // rather than a misleading zero-length fill.
    private function drawQuota(dc as Graphics.Dc, x as Number, labelY as Number,
                               barY as Number, cellW as Number, name as String,
                               pct, suffix as String, accent as Number) as Void {
        dc.setColor(G_MUTED, Graphics.COLOR_TRANSPARENT);
        dc.drawText(x, labelY, Graphics.FONT_GLANCE, name,
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
        // Measured rather than right-aligned to the cell: at this width the
        // two ran flush together, and a space belongs between a label and its
        // value however narrow the column gets.
        var nameW = dc.getTextWidthInPixels(name + " ", Graphics.FONT_GLANCE);

        var label = "--";
        var p = 0;
        if (pct != null) {
            p = pct as Number;
            if (p > 100) { p = 100; }
            if (p < 0) { p = 0; }
            label = p.format("%d") + suffix;
        }

        dc.setColor(pct != null ? G_TEXT : G_MUTED, Graphics.COLOR_TRANSPARENT);
        dc.drawText(x + nameW, labelY, Graphics.FONT_GLANCE, label,
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);

        var barH = 7;
        var r = barH / 2;
        var top = barY - (barH / 2);

        dc.setColor(G_TRACK, Graphics.COLOR_TRANSPARENT);
        dc.fillRoundedRectangle(x, top, cellW, barH, r);

        if (pct != null && p > 0) {
            var fillW = (cellW * p / 100.0).toNumber();
            // A non-zero quota always shows something: a sliver the eye can
            // find beats a bar that looks identical to empty.
            if (fillW < barH) { fillW = barH; }
            dc.setColor(Snap.pctColor(p, accent), Graphics.COLOR_TRANSPARENT);
            dc.fillRoundedRectangle(x, top, fillW, barH, r);
        }
    }
}
