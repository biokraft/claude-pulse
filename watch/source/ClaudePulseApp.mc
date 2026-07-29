import Toybox.Application;
import Toybox.Background;
import Toybox.Lang;
import Toybox.System;
import Toybox.Time;
import Toybox.WatchUi;

(:background)
class ClaudePulseApp extends Application.AppBase {
    function initialize() { AppBase.initialize(); }

    function onStart(state as Dictionary?) as Void {
        registerTemporalEvent();
    }

    function onSettingsChanged() as Void {
        registerTemporalEvent();
        if (Toybox has :WatchUi) {
            WatchUi.requestUpdate();
        }
    }

    function registerTemporalEvent() as Void {
        Background.registerForTemporalEvent(new Time.Duration(300));
    }

    (:background)
    function getServiceDelegate() as [System.ServiceDelegate] {
        return [new FetchDelegate()];
    }

    function onBackgroundData(data as Application.PersistableType) as Void {
        if (data != null) {
            Snap.save(data as Dictionary, Time.now().value());
        }
        WatchUi.requestUpdate();
    }

    function getInitialView() as [Views] or [Views, InputDelegates] {
        return [new RingsView(), new PageDelegate(0)];
    }

    (:glance)
    function getGlanceView() as [WatchUi.GlanceView] or [WatchUi.GlanceView, WatchUi.GlanceViewDelegate] or Null {
        return [new GlanceView()];
    }
}
