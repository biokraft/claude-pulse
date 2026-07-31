import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Application.Properties;

class RingsView extends WatchUi.View {
    function initialize() { View.initialize(); }

    // Pure layout helper: outer ring 140px (radius 70) at 400px reference,
    // ring separation 32px at 400px reference. Scales with width w.
    static function ringGeometry(w as Number) as Dictionary {
        var scale = w / 400.0;
        var radius = (70 * scale).toNumber();
        var halfGap = (16 * scale).toNumber();
        var y = (w / 2).toNumber();
        var cx = y;
        return {
            "radius" => radius,
            "y" => y,
            "leftX" => cx - (radius + halfGap),
            "rightX" => cx + (radius + halfGap)
        };
    }

    function onUpdate(dc as Graphics.Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();

        var w = dc.getWidth();
        var accent = Properties.getValue("accentColor") as Number;
        if (accent == null) { accent = 0xCC7A56; }

        var nowEpoch = Time.now().value();
        var stored = Snap.load();

        dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
        dc.drawText(w / 2, (dc.getHeight() * 0.10).toNumber(), Graphics.FONT_XTINY, "CLAUDE USAGE",
            Graphics.TEXT_JUSTIFY_CENTER);

        var fivePct = 0;
        var sevenPct = 0;
        var stale = true;

        if (stored != null) {
            var d = stored["d"] as Dictionary;
            if (d != null) {
                var fp = d["five_hour_pct"];
                var sp = d["seven_day_pct"];
                if (fp != null) { fivePct = fp as Number; }
                if (sp != null) { sevenPct = sp as Number; }
            }
            stale = Snap.isStale(stored, nowEpoch);
        }

        var geo = ringGeometry(w);
        var radius = geo["radius"] as Number;
        var y = geo["y"] as Number;
        var penWidth = (10 * w / 400.0).toNumber();
        if (penWidth < 1) { penWidth = 1; }

        drawRing(dc, geo["leftX"] as Number, y, radius, penWidth, fivePct, accent, stale, "5H");
        drawRing(dc, geo["rightX"] as Number, y, radius, penWidth, sevenPct, accent, stale, "7D");

        if (stored != null && stale) {
            var mins = Snap.ageMinutes(stored, nowEpoch);
            dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
            dc.drawText(w / 2, dc.getHeight() - (dc.getHeight() * 0.16).toNumber(), Graphics.FONT_XTINY,
                "synced " + mins + "m ago", Graphics.TEXT_JUSTIFY_CENTER);
        }
    }

    function drawRing(dc as Graphics.Dc, cx as Number, cy as Number, radius as Number,
        penWidth as Number, pct as Number, accent as Number, stale as Boolean, label as String) as Void {

        if (pct > 100) { pct = 100; }
        if (pct < 0) { pct = 0; }

        var trackColor = 0x333333;
        var valueColor = stale ? LT_GRAY : Snap.pctColor(pct, accent);

        dc.setPenWidth(penWidth);
        dc.setColor(trackColor, Graphics.COLOR_BLACK);
        dc.drawArc(cx, cy, radius, Graphics.ARC_CLOCKWISE, 90, 90.001);

        if (pct > 0) {
            dc.setColor(valueColor, Graphics.COLOR_BLACK);
            dc.drawArc(cx, cy, radius, Graphics.ARC_CLOCKWISE, 90, 90 - 360 * pct / 100.0);
        }

        dc.setColor(stale ? LT_GRAY : Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.drawText(cx, cy, Graphics.FONT_MEDIUM, pct.format("%d"),
            Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER);

        dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
        dc.drawText(cx, cy + radius + penWidth, Graphics.FONT_XTINY, label,
            Graphics.TEXT_JUSTIFY_CENTER);
    }
}
