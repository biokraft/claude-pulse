import Toybox.Test;
import Toybox.Lang;

(:test)
function testRingGeometryFull(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(400);
    Test.assertEqual(g["radius"], 46);
    Test.assertEqual(g["y"], 200);
    Test.assertEqual(g["leftX"], 200 - 66);
    Test.assertEqual(g["rightX"], 200 + 66);
    return true;
}

(:test)
function testRingGeometryScaled(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(260);
    Test.assertEqual(g["radius"], 29);
    Test.assertEqual(g["y"], 130);
    Test.assertEqual(g["leftX"], 130 - 42);
    Test.assertEqual(g["rightX"], 130 + 42);
    return true;
}
