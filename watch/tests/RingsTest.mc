import Toybox.Test;
import Toybox.Lang;

(:test)
function testRingGeometryFull(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(400);
    Test.assertEqual(g["radius"], 70);
    Test.assertEqual(g["y"], 200);
    Test.assertEqual(g["leftX"], 200 - (70 + 16));
    Test.assertEqual(g["rightX"], 200 + (70 + 16));
    return true;
}

(:test)
function testRingGeometryScaled(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(260);
    Test.assertEqual(g["radius"], 45);
    Test.assertEqual(g["y"], 130);
    Test.assertEqual(g["leftX"], 130 - (45 + 10));
    Test.assertEqual(g["rightX"], 130 + (45 + 10));
    return true;
}
