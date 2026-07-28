import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Time.Gregorian;
import Toybox.Application.Properties;

// Page 2: mascot + active job count + per-window (5H/7D) quota rows.
class DetailView extends WatchUi.View {
    function initialize() { View.initialize(); }

    // Pure-ish helper: pose symbol -> drawable resource id. Exposed static for testability.
    static function spriteForPose(pose as Symbol) as ResourceId {
        if (pose == :celebrate) { return Rez.Drawables.ClawdHappy; }
        if (pose == :annoyed) { return Rez.Drawables.ClawdError; }
        if (pose == :working) { return Rez.Drawables.ClawdHeadphonesGroove; }
        if (pose == :sleeping) { return Rez.Drawables.ClawdCollapseSleep; }
        if (pose == :idleCollapse) { return Rez.Drawables.ClawdIdleCollapse; }
        if (pose == :idleDoze) { return Rez.Drawables.ClawdIdleDoze; }
        if (pose == :idleYawn) { return Rez.Drawables.ClawdIdleYawn; }
        return Rez.Drawables.ClawdIdleLook; // :idle and any unknown fallback
    }

    static function labelForPose(pose as Symbol) as String {
        if (pose == :celebrate) { return "celebrating"; }
        if (pose == :annoyed) { return "annoyed"; }
        if (pose == :working) { return "working"; }
        if (pose == :sleeping) { return "sleeping"; }
        if (pose == :idleCollapse) { return "collapsed"; }
        if (pose == :idleDoze) { return "dozing"; }
        if (pose == :idleYawn) { return "yawning"; }
        return "idle";
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
        var fiveResetsAt = 0;
        var sevenResetsAt = 0;
        var isActive = false;
        var activeCount = 0;
        var stale = true;
        var hasData = stored != null;

        if (hasData) {
            var d = stored["d"] as Dictionary;
            if (d != null) {
                if (d["five_hour_pct"] != null) { fivePct = d["five_hour_pct"] as Number; }
                if (d["seven_day_pct"] != null) { sevenPct = d["seven_day_pct"] as Number; }
                if (d["is_active"] != null) { isActive = d["is_active"] as Boolean; }
                if (d["active_count"] != null) { activeCount = d["active_count"] as Number; }
                var fra = d["five_hour_resets_at"];
                if (fra != null) { fiveResetsAt = Snap.parseIso8601(fra as String); }
                var sra = d["seven_day_resets_at"];
                if (sra != null) { sevenResetsAt = Snap.parseIso8601(sra as String); }
            }
            stale = Snap.isStale(stored, nowEpoch);
        }

        var inactiveSecs = isActive ? 0 : (nowEpoch - (hasData ? (stored["fetchedEpoch"] as Number) : nowEpoch));
        var info = Gregorian.info(Time.now(), Time.FORMAT_SHORT);
        var hourOfDay = info.hour;
        var pose = Pose.compute(fivePct, sevenPct, isActive, hourOfDay, inactiveSecs, 0, nowEpoch);

        var poseColor = stale ? LT_GRAY : (pose == :annoyed ? Snap.WARN_COLOR : accent);

        var cy = (52 * scale).toNumber();
        var spriteSize = (52 * scale).toNumber();
        dc.drawBitmap(w / 2 - spriteSize / 2, cy - spriteSize / 2, WatchUi.loadResource(spriteForPose(pose)));

        var labelY = cy + spriteSize / 2 + (4 * scale).toNumber();
        dc.setColor(poseColor, Graphics.COLOR_BLACK);
        dc.drawText(w / 2, labelY, Graphics.FONT_XTINY, labelForPose(pose), Graphics.TEXT_JUSTIFY_CENTER);

        var jobsY = labelY + (18 * scale).toNumber();
        var jobsText = activeCount.format("%d") + (activeCount == 1 ? " job running" : " jobs running");
        dc.drawText(w / 2, jobsY, Graphics.FONT_XTINY, jobsText, Graphics.TEXT_JUSTIFY_CENTER);

        var rowWidth = (w * 0.65).toNumber();
        var rowX = w / 2 - rowWidth / 2;
        var row1Y = jobsY + (28 * scale).toNumber();
        var row2Y = row1Y + (46 * scale).toNumber();

        drawRow(dc, rowX, row1Y, rowWidth, scale, "5H", fivePct, fiveResetsAt, nowEpoch, accent, stale, hasData);
        drawRow(dc, rowX, row2Y, rowWidth, scale, "7D", sevenPct, sevenResetsAt, nowEpoch, accent, stale, hasData);

        if (hasData && stale) {
            var mins = Snap.ageMinutes(stored, nowEpoch);
            dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
            dc.drawText(w / 2, h - (16 * scale).toNumber(), Graphics.FONT_XTINY,
                "synced " + mins + "m ago", Graphics.TEXT_JUSTIFY_CENTER);
        }
    }

    function drawRow(dc as Graphics.Dc, x as Number, y as Number, rowWidth as Number, scale as Float,
            label as String, pct as Number, resetsAt as Number, nowEpoch as Number, accent as Number,
            stale as Boolean, hasData as Boolean) as Void {

        var color = stale ? LT_GRAY : Snap.pctColor(pct, accent);

        dc.setColor(stale ? LT_GRAY : Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.drawText(x, y, Graphics.FONT_XTINY, label, Graphics.TEXT_JUSTIFY_LEFT);
        dc.setColor(color, Graphics.COLOR_BLACK);
        dc.drawText(x + rowWidth, y, Graphics.FONT_XTINY, pct.format("%d") + "%", Graphics.TEXT_JUSTIFY_RIGHT);

        var barY = y + (16 * scale).toNumber();
        var barH = (6 * scale).toNumber();
        if (barH < 2) { barH = 2; }
        var barR = barH / 2;

        dc.setColor(0x333333, Graphics.COLOR_BLACK);
        dc.fillRoundedRectangle(x, barY, rowWidth, barH, barR);

        var fillW = (rowWidth * pct / 100.0).toNumber();
        if (fillW > 0) {
            dc.setColor(color, Graphics.COLOR_BLACK);
            dc.fillRoundedRectangle(x, barY, fillW, barH, barR);
        }

        var captionY = barY + barH + (4 * scale).toNumber();
        dc.setColor(LT_GRAY, Graphics.COLOR_BLACK);
        var caption = hasData ? "resets in " + Snap.countdown(resetsAt, nowEpoch) : "resets in --";
        dc.drawText(x + rowWidth / 2, captionY, Graphics.FONT_XTINY, caption, Graphics.TEXT_JUSTIFY_CENTER);
    }
}
