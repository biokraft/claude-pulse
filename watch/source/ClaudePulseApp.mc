import Toybox.Application;
import Toybox.Lang;
import Toybox.WatchUi;

(:background)
class ClaudePulseApp extends Application.AppBase {
    function initialize() { AppBase.initialize(); }

    function getInitialView() as [Views] or [Views, InputDelegates] {
        return [new GlanceView()]; // replaced by page carousel in Task 8
    }

    (:glance)
    function getGlanceView() as [WatchUi.GlanceView] or [WatchUi.GlanceView, WatchUi.GlanceViewDelegate] or Null {
        return [new GlanceView()];
    }
}
