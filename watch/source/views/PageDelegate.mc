import Toybox.WatchUi;
import Toybox.Lang;

// Carousel navigation across the 3 main pages: Rings(0) -> Detail(1) -> Cost(2).
// UP/DOWN buttons and touch swipe both page through; BACK returns to page 1
// (Rings) from pages 2/3, then pops the app from page 1 (system default).
class PageDelegate extends WatchUi.BehaviorDelegate {
    private var _page as Number;

    function initialize(page as Number) {
        BehaviorDelegate.initialize();
        _page = page;
    }

    function onNextPage() as Boolean {
        switchToPage((_page + 1) % 3, WatchUi.SLIDE_LEFT);
        return true;
    }

    function onPreviousPage() as Boolean {
        switchToPage((_page + 2) % 3, WatchUi.SLIDE_RIGHT);
        return true;
    }

    function onBack() as Boolean {
        if (_page != 0) {
            switchToPage(0, WatchUi.SLIDE_RIGHT);
            return true;
        }
        return false; // let system pop the app
    }

    private function switchToPage(page as Number, dir as WatchUi.SlideType) as Void {
        var view;
        if (page == 0) {
            view = new RingsView();
        } else if (page == 1) {
            view = new DetailView();
        } else {
            view = new CostView();
        }
        WatchUi.switchToView(view, new PageDelegate(page), dir);
    }
}
