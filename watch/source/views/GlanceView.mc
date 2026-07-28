import Toybox.WatchUi;
import Toybox.Graphics;

(:glance)
class GlanceView extends WatchUi.GlanceView {
    function initialize() { GlanceView.initialize(); }
    function onUpdate(dc as Graphics.Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();
        dc.drawText(0, dc.getHeight() / 2, Graphics.FONT_GLANCE, "CLAUDE --",
            Graphics.TEXT_JUSTIFY_LEFT | Graphics.TEXT_JUSTIFY_VCENTER);
    }
}
