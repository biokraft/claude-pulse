import Toybox.Test;
import Toybox.Lang;

(:test)
function testRingGeometryFull(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(400);
    Test.assertEqual(g["radius"], 52);
    Test.assertEqual(g["y"], 200);
    Test.assertEqual(g["leftX"], 200 - (52 + 14));
    Test.assertEqual(g["rightX"], 200 + (52 + 14));
    return true;
}

(:test)
function testRingGeometryScaled(logger as Test.Logger) as Boolean {
    var g = RingsView.ringGeometry(260);
    Test.assertEqual(g["radius"], 33);
    Test.assertEqual(g["y"], 130);
    Test.assertEqual(g["leftX"], 130 - (33 + 9));
    Test.assertEqual(g["rightX"], 130 + (33 + 9));
    return true;
}
