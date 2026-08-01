import Toybox.WatchUi;
import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Time;
import Toybox.Time.Gregorian;
import Toybox.Application.Properties;

// Page 2: mascot + active job count + per-window quota rows. Geometry mirrors the
// design mockup at its 400px reference size (sprite 52, 260-wide rows, gap 8/14).
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

        if (stored != null) {
            var d = stored["d"] as Dictionary?;
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

        var fetchedEpoch = nowEpoch;
        if (stored != null) { fetchedEpoch = stored["fetchedEpoch"] as Number; }
        var inactiveSecs = isActive ? 0 : (nowEpoch - fetchedEpoch);
        var info = Gregorian.info(Time.now(), Time.FORMAT_SHORT);
        var hourOfDay = info.hour;
        var pose = Pose.compute(fivePct, sevenPct, isActive, hourOfDay, inactiveSecs, 0, nowEpoch);

        var poseColor = stale ? Chrome.DIM : (pose == :annoyed ? Snap.WARN_COLOR : accent);

        var fh = dc.getFontHeight(Graphics.FONT_XTINY);
        // The drawables are cropped to the mascot itself (50x33 source, see
        // scripts/crop-sprites.py), so the drawn box is all mascot and the
        // aspect ratio has to be preserved.
        var spriteW = (80 * scale).toNumber();
        var spriteH = (spriteW * 33 / 50.0).toNumber();
        var gap = (8 * scale).toNumber();
        if (gap < 2) { gap = 2; }

        var rowHeight = fh + (6 * scale).toNumber() + (6 * scale).toNumber() + (4 * scale).toNumber() + fh;
        var rowGap = (14 * scale).toNumber();
        var rowsH = rowHeight * 2 + rowGap;

        // Every element below is real ink now, so centring the sum of their
        // heights centres what the wearer actually sees.
        var total = spriteH + gap + fh + gap + rowsH;
        var y = (h - total) / 2;

        dc.drawScaledBitmap(w / 2 - spriteW / 2, y, spriteW, spriteH,
            WatchUi.loadResource(spriteForPose(pose)) as WatchUi.BitmapResource);
        y += spriteH + gap;

        dc.setColor(poseColor, Graphics.COLOR_BLACK);
        var jobsText = activeCount.format("%d") + (activeCount == 1 ? " job running" : " jobs running");
        dc.drawText(w / 2, y, Graphics.FONT_XTINY, jobsText, Graphics.TEXT_JUSTIFY_CENTER);
        y += fh + gap;

        var blockWidth = (260 * scale).toNumber();
        var blockX = w / 2 - blockWidth / 2;

        var fiveColor = stale ? Chrome.DIM : Snap.pctColor(fivePct, accent);
        var sevenColor = stale ? Chrome.DIM : Snap.pctColor(sevenPct, accent);

        drawRow(dc, blockX, y, blockWidth, scale, "5 hour", fiveColor, fivePct, fiveResetsAt,
            nowEpoch, stale, hasData);
        drawRow(dc, blockX, y + rowHeight + rowGap, blockWidth, scale, "7 day", sevenColor, sevenPct,
            sevenResetsAt, nowEpoch, stale, hasData);

        if (stored != null && stale) {
            var mins = Snap.ageMinutes(stored, nowEpoch);
            dc.setColor(Chrome.DIM, Graphics.COLOR_BLACK);
            dc.drawText(w / 2, h - (44 * scale).toNumber() - fh, Graphics.FONT_XTINY,
                "synced " + mins + "m ago", Graphics.TEXT_JUSTIFY_CENTER);
        }

        Chrome.drawPageDots(dc, 1, accent);
    }

    // One quota row: 14px rounded-square bullet, then label/percent, bar and caption
    // in the remaining width.
    function drawRow(dc as Graphics.Dc, x as Number, y as Number, blockWidth as Number, scale as Float,
            label as String, color as Number, pct as Number, resetsAt as Number, nowEpoch as Number,
            stale as Boolean, hasData as Boolean) as Void {

        if (pct > 100) { pct = 100; }
        if (pct < 0) { pct = 0; }

        var fh = dc.getFontHeight(Graphics.FONT_XTINY);
        var bulletSize = (14 * scale).toNumber();
        if (bulletSize < 4) { bulletSize = 4; }
        var bulletR = (3 * scale).toNumber();
        if (bulletR < 1) { bulletR = 1; }

        dc.setColor(color, Graphics.COLOR_BLACK);
        dc.fillRoundedRectangle(x, y + fh / 2 - bulletSize / 2, bulletSize, bulletSize, bulletR);

        var contentX = x + bulletSize + (14 * scale).toNumber();
        var contentW = x + blockWidth - contentX;

        dc.setColor(stale ? Chrome.DIM : Chrome.TEXT_PRIMARY, Graphics.COLOR_BLACK);
        dc.drawText(contentX, y, Graphics.FONT_XTINY, label, Graphics.TEXT_JUSTIFY_LEFT);
        dc.drawText(contentX + contentW, y, Graphics.FONT_XTINY, pct.format("%d") + "%",
            Graphics.TEXT_JUSTIFY_RIGHT);

        var barY = y + fh + (6 * scale).toNumber();
        var barH = (6 * scale).toNumber();
        if (barH < 2) { barH = 2; }
        var barR = barH / 2;

        dc.setColor(Chrome.TRACK, Graphics.COLOR_BLACK);
        dc.fillRoundedRectangle(contentX, barY, contentW, barH, barR);

        var fillW = (contentW * pct / 100.0).toNumber();
        if (fillW > 0) {
            dc.setColor(color, Graphics.COLOR_BLACK);
            dc.fillRoundedRectangle(contentX, barY, fillW, barH, barR);
        }

        var captionY = barY + barH + (4 * scale).toNumber();
        dc.setColor(Chrome.DIM, Graphics.COLOR_BLACK);
        var caption = hasData ? "resets in " + Snap.countdown(resetsAt, nowEpoch) : "resets in --";
        dc.drawText(contentX, captionY, Graphics.FONT_XTINY, caption, Graphics.TEXT_JUSTIFY_LEFT);
    }
}
