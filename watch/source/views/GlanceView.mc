import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Application.Properties;

const LT_GRAY = 0xAAAAAA;

(:glance)
class GlanceView extends WatchUi.GlanceView {
    function initialize() { GlanceView.initialize(); }

    function onUpdate(dc as Graphics.Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();

        var accent = Properties.getValue("accentColor") as Number;
        var nowEpoch = Time.now().value();
        var stored = Snap.load();

        var line2 = "-- · --";
        var color = accent;

        if (stored != null) {
            var d = stored["d"] as Dictionary;
            var fivePct = (d != null) ? d["five_hour_pct"] : null;
            var sevenPct = (d != null) ? d["seven_day_pct"] : null;
            var fiveStr = (fivePct != null) ? (fivePct as Number).format("%d") + "%" : "--";
            var sevenStr = (sevenPct != null) ? (sevenPct as Number).format("%d") + "%" : "--";
            line2 = fiveStr + " · " + sevenStr;
            if (fivePct != null) { color = Snap.pctColor(fivePct as Number, accent); }
            if (sevenPct != null) {
                var sevenColor = Snap.pctColor(sevenPct as Number, accent);
                if (sevenColor != accent) { color = sevenColor; }
            }
        }

        if (stored != null && Snap.isStale(stored, nowEpoch)) {
            line2 += "!";
            color = LT_GRAY;
        }

        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.drawText(0, dc.getHeight() / 4, Graphics.FONT_GLANCE, "CLAUDE",
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);

        dc.setColor(color, Graphics.COLOR_BLACK);
        dc.drawText(0, (dc.getHeight() * 3) / 4, Graphics.FONT_GLANCE, line2,
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
    }
}
