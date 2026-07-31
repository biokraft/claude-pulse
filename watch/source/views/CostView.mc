import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Application.Properties;

// Page 3: today's cost figure + token caption + 7-day bar chart.
class CostView extends WatchUi.View {
    function initialize() { View.initialize(); }

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

        var costUsd = 0.0;
        var tokens = 0;
        var daily = [] as Array<Dictionary>;
        var stale = true;
        var hasData = stored != null;

        if (hasData) {
            var d = stored["d"] as Dictionary;
            if (d != null) {
                if (d["today_cost_usd"] != null) { costUsd = (d["today_cost_usd"] as Numeric).toFloat(); }
                if (d["today_tokens"] != null) { tokens = d["today_tokens"] as Number; }
                if (d["daily"] != null) { daily = d["daily"] as Array<Dictionary>; }
            }
            stale = Snap.isStale(stored, nowEpoch);
        }

        var color = stale ? LT_GRAY : Graphics.COLOR_WHITE;

        dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
        dc.drawText(w / 2, (h * 0.10).toNumber(), Graphics.FONT_XTINY, "TODAY'S COST",
            Graphics.TEXT_JUSTIFY_CENTER);

        // Number fonts on many devices only carry digits and a few symbols, so
        // the "$" prefix is drawn separately in a text font next to the figure.
        var numText = hasData ? costUsd.format("%.2f") : "--";
        var numW = dc.getTextWidthInPixels(numText, Graphics.FONT_NUMBER_MEDIUM);
        var dollarW = dc.getTextWidthInPixels("$", Graphics.FONT_MEDIUM);
        var costCenterY = (h * 0.34).toNumber();
        var startX = w / 2 - (numW + dollarW) / 2;
        dc.setColor(color, Graphics.COLOR_BLACK);
        dc.drawText(startX, costCenterY, Graphics.FONT_MEDIUM, "$",
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
        dc.drawText(startX + dollarW, costCenterY, Graphics.FONT_NUMBER_MEDIUM, numText,
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);

        var captionText = hasData ? Chart.formatTokens(tokens) + " tokens" : "-- tokens";
        var captionY = costCenterY + dc.getFontHeight(Graphics.FONT_NUMBER_MEDIUM) / 2 + (4 * scale).toNumber();
        dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
        dc.drawText(w / 2, captionY, Graphics.FONT_XTINY, captionText, Graphics.TEXT_JUSTIFY_CENTER);

        if (daily.size() == 7) {
            drawChart(dc, w, h, scale, daily, stale, accent);
        }

        if (hasData && stale) {
            var mins = Snap.ageMinutes(stored, nowEpoch);
            dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
            dc.drawText(w / 2, h - (h * 0.13).toNumber(), Graphics.FONT_XTINY,
                "synced " + mins + "m ago", Graphics.TEXT_JUSTIFY_CENTER);
        }
    }

    function drawChart(dc as Graphics.Dc, w as Number, h as Number, scale as Float,
            daily as Array<Dictionary>, stale as Boolean, accent as Number) as Void {

        var maxPx = (56 * scale).toNumber();
        var barW = (12 * scale).toNumber();
        if (barW < 1) { barW = 1; }
        var gap = (6 * scale).toNumber();
        if (gap < 1) { gap = 1; }

        var n = daily.size();
        var totalW = n * barW + (n - 1) * gap;
        var startX = w / 2 - totalW / 2;
        var baseY = h - (h * 0.18).toNumber();

        var heights = Chart.barHeights(daily, maxPx);

        for (var i = 0; i < n; i += 1) {
            var barH = heights[i];
            var x = startX + i * (barW + gap);
            var isToday = i == n - 1;
            var barColor = stale ? LT_GRAY : (isToday ? accent : 0x555555);

            dc.setColor(0x333333, Graphics.COLOR_BLACK);
            dc.fillRectangle(x, baseY - maxPx, barW, maxPx);

            if (barH > 0) {
                dc.setColor(barColor, Graphics.COLOR_BLACK);
                dc.fillRectangle(x, baseY - barH, barW, barH);
            }
        }
    }
}
