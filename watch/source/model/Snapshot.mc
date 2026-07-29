import Toybox.Lang;
import Toybox.Application;
import Toybox.Time;
import Toybox.Time.Gregorian;

(:glance,:background)
module Snap {
    const STALE_SECS = 900;
    const WARN_PCT = 85;
    const WARN_COLOR = 0xC24B3A;

    function save(data as Dictionary, nowEpoch as Number) as Void {
        Application.Storage.setValue("snap", {"d" => data, "fetchedEpoch" => nowEpoch});
    }

    function load() as Dictionary? {
        return Application.Storage.getValue("snap") as Dictionary?;
    }

    function isStale(stored as Dictionary?, nowEpoch as Number) as Boolean {
        if (stored == null) { return true; }
        var d = stored["d"] as Dictionary;
        if (d != null && d["stale"] == true) { return true; }
        return nowEpoch - (stored["fetchedEpoch"] as Number) > STALE_SECS;
    }

    function ageMinutes(stored as Dictionary?, nowEpoch as Number) as Number {
        if (stored == null) { return -1; }
        return (nowEpoch - (stored["fetchedEpoch"] as Number)) / 60;
    }

    function countdown(resetsAtEpoch as Number, nowEpoch as Number) as String {
        var secs = resetsAtEpoch - nowEpoch;
        if (secs <= 0) { return "now"; }
        var days = secs / 86400;
        if (days >= 1) {
            return days.format("%d") + "d " + ((secs % 86400) / 3600).format("%d") + "h";
        }
        return (secs / 3600).format("%d") + "h " + ((secs % 3600) / 60).format("%d") + "m";
    }

    function parseIso8601(s as String) as Number {
        // expects YYYY-MM-DDTHH:MM:SSZ
        if (s == null || s.length() < 20) { return 0; }
        var y = s.substring(0, 4).toNumber();
        var mo = s.substring(5, 7).toNumber();
        var da = s.substring(8, 10).toNumber();
        var h = s.substring(11, 13).toNumber();
        var mi = s.substring(14, 16).toNumber();
        var se = s.substring(17, 19).toNumber();
        if (y == null || mo == null || da == null || h == null || mi == null || se == null) { return 0; }
        if (y < 1970) { return 0; }
        var opts = {:year => y, :month => mo, :day => da, :hour => h, :minute => mi, :second => se};
        var mom = Gregorian.moment(opts);
        return mom.value();
    }

    function pctColor(pct as Number, accent as Number) as Number {
        return pct >= WARN_PCT ? WARN_COLOR : accent;
    }
}
