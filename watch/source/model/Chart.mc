import Toybox.Lang;

// Pure chart math for CostView. No (:glance) needed — only used from
// normal app partition (page 3).
module Chart {
    function barHeights(daily as Array<Dictionary>, maxPx as Number) as Array<Number> {
        var n = daily.size();
        var values = new [n];
        var maxVal = 0.0;
        for (var i = 0; i < n; i += 1) {
            var v = daily[i]["cost_usd"] as Numeric;
            var f = v.toFloat();
            values[i] = f;
            if (f > maxVal) { maxVal = f; }
        }

        var heights = new [n];
        for (var i = 0; i < n; i += 1) {
            if (maxVal <= 0.0 || values[i] <= 0.0) {
                heights[i] = 0;
                continue;
            }
            var px = (values[i] / maxVal * maxPx).toNumber();
            if (px < 2) { px = 2; }
            heights[i] = px;
        }
        return heights;
    }

    function formatTokens(n as Number) as String {
        if (n < 1000) {
            return n.format("%d");
        }
        if (n < 1000000) {
            return (n / 1000).format("%d") + "K";
        }
        return (n / 1000000.0).format("%.1f") + "M";
    }
}
