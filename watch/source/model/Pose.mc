import Toybox.Lang;

module Pose {
    function compute(fiveHourPct as Number, sevenDayPct as Number, isActive as Boolean,
            hourOfDay as Number, inactiveSecs as Number,
            celebrateUntilEpoch as Number, nowEpoch as Number) as Symbol {
        if (nowEpoch < celebrateUntilEpoch) { return :celebrate; }
        if (fiveHourPct >= Snap.WARN_PCT || sevenDayPct >= Snap.WARN_PCT) { return :annoyed; }
        if (isActive) { return :working; }
        var night = hourOfDay >= 0 && hourOfDay < 6;
        if (night) {
            if (inactiveSecs >= 63) { return :sleeping; }
            if (inactiveSecs >= 60) { return :idleCollapse; }
            if (inactiveSecs >= 40) { return :idleDoze; }
            if (inactiveSecs >= 20) { return :idleYawn; }
        }
        return :idle;
    }
}
