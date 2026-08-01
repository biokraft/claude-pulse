import Toybox.Test;
import Toybox.Lang;

(:test)
function testRingGeometryFull(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(400);
    Test.assertEqual((g["radius"] as Number), 46);
    Test.assertEqual((g["y"] as Number), 200);
    Test.assertEqual((g["leftX"] as Number), 200 - 66);
    Test.assertEqual((g["rightX"] as Number), 200 + 66);
    return true;
}

(:test)
function testRingGeometryScaled(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(260);
    Test.assertEqual((g["radius"] as Number), 29);
    Test.assertEqual((g["y"] as Number), 130);
    Test.assertEqual((g["leftX"] as Number), 130 - 42);
    Test.assertEqual((g["rightX"] as Number), 130 + 42);
    return true;
}
