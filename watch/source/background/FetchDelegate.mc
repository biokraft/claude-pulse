import Toybox.Background;
import Toybox.System;
import Toybox.Communications;
import Toybox.Application;
import Toybox.Lang;

(:background)
class FetchDelegate extends System.ServiceDelegate {
    function initialize() {
        ServiceDelegate.initialize();
    }

    function onTemporalEvent() as Void {
        var url = Application.Properties.getValue("relayUrl") as String;
        var token = Application.Properties.getValue("relayToken") as String;
        if (url == null || url.length() == 0) {
            Background.exit(null);
            return;
        }
        while (url.length() > 0 && url.substring(url.length() - 1, url.length()).equals("/")) {
            url = url.substring(0, url.length() - 1);
        }
        Communications.makeWebRequest(
            url + "/api/v1/snapshot",
            {"token" => token},
            {:method => Communications.HTTP_REQUEST_METHOD_GET,
             :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON},
            method(:onResponse));
    }

    function onResponse(code as Number, data as Dictionary or String or Null) as Void {
        Background.exit(code == 200 ? data : null);
    }
}
