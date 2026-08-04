import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Application.Properties;

// Page 1: quota rings + activity status. Geometry mirrors the design mockup at
// its 400px reference size (outer ring 104, hole 80, pair gap 28).
class RingsView extends WatchUi.View {
    function initialize() { View.initialize(); }

    // Pure layout helper: arc radius 46 and half-gap 66 at the 400px reference.
    static function ringGeometry(w as Number) as Dictionary {
        var scale = w / 400.0;
        var radius = (46 * scale).toNumber();
        var cx = (w / 2).toNumber();
        return {
            "radius" => radius,
            "y" => cx,
            "leftX" => cx - (66 * scale).toNumber(),
            "rightX" => cx + (66 * scale).toNumber()
        };
    }

    function onUpdate(dc as Graphics.Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();

        var w = dc.getWidth();
        var h = dc.getHeight();
        var scale = w / 400.0;
        var accent = Properties.getValue("accentColor") as Number;
        if (accent == null) { accent = 0xCC7A56; }

        var nowEpoch = Time.now().value();
        var stored = Snap.load();

        var fivePct = 0;
        var sevenPct = 0;
        var isActive = false;
        var stale = true;

        if (stored != null) {
            var d = stored["d"] as Dictionary;
            if (d != null) {
                var fp = d["five_hour_pct"];
                var sp = d["seven_day_pct"];
                if (fp != null) { fivePct = fp as Number; }
                if (sp != null) { sevenPct = sp as Number; }
                if (d["is_active"] != null) { isActive = d["is_active"] as Boolean; }
            }
            stale = Snap.isStale(stored, nowEpoch);
        }

        var fh = dc.getFontHeight(Graphics.FONT_XTINY);
        var gap = (14 * scale).toNumber();
        var outerR = (52 * scale).toNumber();
        var penWidth = (12 * scale).toNumber();
        if (penWidth < 2) { penWidth = 2; }
        var arcR = (46 * scale).toNumber();
        var dotR = (5 * scale).toNumber();
        if (dotR < 2) { dotR = 2; }

        var ringBlockH = outerR * 2 + (8 * scale).toNumber() + fh;
        var statusH = fh > dotR * 2 ? fh : dotR * 2;
        var total = fh + gap + ringBlockH + gap + statusH;
        var top = (h - total) / 2;

        dc.setColor(Chrome.MUTED, Graphics.COLOR_BLACK);
        dc.drawText(w / 2, top, Graphics.FONT_XTINY, "CLAUDE USAGE", Graphics.TEXT_JUSTIFY_CENTER);

        var ringY = top + fh + gap + outerR;
        var leftX = w / 2 - (66 * scale).toNumber();
        var rightX = w / 2 + (66 * scale).toNumber();

        drawRing(dc, leftX, ringY, arcR, outerR, penWidth, scale, fivePct, accent, stale, "5H");
        drawRing(dc, rightX, ringY, arcR, outerR, penWidth, scale, sevenPct, accent, stale, "7D");

        var statusColor = stale ? Chrome.DIM : (isActive ? accent : Chrome.DIM);
        var statusText = isActive ? "ACTIVE" : "IDLE";
        var statusCenterY = top + total - statusH / 2;
        var textW = dc.getTextWidthInPixels(statusText, Graphics.FONT_XTINY);
        var statusGap = (6 * scale).toNumber();
        if (statusGap < 2) { statusGap = 2; }
        var startX = w / 2 - (dotR * 2 + statusGap + textW) / 2;

        dc.setColor(statusColor, Graphics.COLOR_BLACK);
        dc.fillCircle(startX + dotR, statusCenterY, dotR);
        dc.drawText(startX + dotR * 2 + statusGap, statusCenterY, Graphics.FONT_XTINY, statusText,
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);

        if (stored != null && stale) {
            var mins = Snap.ageMinutes(stored, nowEpoch);
            dc.setColor(Chrome.DIM, Graphics.COLOR_BLACK);
            dc.drawText(w / 2, h - (44 * scale).toNumber() - fh, Graphics.FONT_XTINY,
                Snap.syncedLabel(mins), Graphics.TEXT_JUSTIFY_CENTER);
        }

        Chrome.drawPageDots(dc, 0, accent);
    }

    function drawRing(dc as Graphics.Dc, cx as Number, cy as Number, arcR as Number, outerR as Number,
        penWidth as Number, scale as Float, pct as Number, accent as Number, stale as Boolean,
        label as String) as Void {

        if (pct > 100) { pct = 100; }
        if (pct < 0) { pct = 0; }

        var valueColor = stale ? Chrome.DIM : Snap.pctColor(pct, accent);

        dc.setPenWidth(penWidth);
        dc.setColor(Chrome.TRACK, Graphics.COLOR_BLACK);
        dc.drawCircle(cx, cy, arcR);

        if (pct > 0) {
            dc.setColor(valueColor, Graphics.COLOR_BLACK);
            dc.drawArc(cx, cy, arcR, Graphics.ARC_CLOCKWISE, 90, 90 - 360 * pct / 100.0);
        }
        dc.setPenWidth(1);

        dc.setColor(stale ? Chrome.DIM : Chrome.TEXT_PRIMARY, Graphics.COLOR_BLACK);
        dc.drawText(cx, cy, Graphics.FONT_TINY, pct.format("%d") + "%",
            Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER);

        dc.setColor(Chrome.MUTED, Graphics.COLOR_BLACK);
        dc.drawText(cx, cy + outerR + (8 * scale).toNumber(), Graphics.FONT_XTINY, label,
            Graphics.TEXT_JUSTIFY_CENTER);
    }
}
