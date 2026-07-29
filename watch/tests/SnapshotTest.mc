import Toybox.Test;
import Toybox.Lang;

(:test)
function testCountdown(logger as Test.Logger) as Boolean {
    var now = 1000000;
    Test.assertEqual(Snap.countdown(now + 2*3600 + 14*60, now), "2h 14m");
    Test.assertEqual(Snap.countdown(now + 4*86400 + 6*3600, now), "4d 6h");
    Test.assertEqual(Snap.countdown(now + 38*60, now), "0h 38m");
    Test.assertEqual(Snap.countdown(now - 5, now), "now");
    return true;
}

(:test)
function testParseIso(logger as Test.Logger) as Boolean {
    Test.assertEqual(Snap.parseIso8601("2026-07-27T10:00:00Z"), 1785146400);
    Test.assertEqual(Snap.parseIso8601("0001-01-01T00:00:00Z"), 0);
    Test.assertEqual(Snap.parseIso8601("garbage"), 0);
    return true;
}

(:test)
function testStale(logger as Test.Logger) as Boolean {
    var now = 2000000;
    Test.assert(Snap.isStale(null, now));
    Test.assert(Snap.isStale({"d" => {"stale" => true}, "fetchedEpoch" => now}, now));
    Test.assert(Snap.isStale({"d" => {"stale" => false}, "fetchedEpoch" => now - 901}, now));
    Test.assert(!Snap.isStale({"d" => {"stale" => false}, "fetchedEpoch" => now - 60}, now));
    return true;
}

(:test)
function testPctColor(logger as Test.Logger) as Boolean {
    Test.assertEqual(Snap.pctColor(84, 0xCC7A56), 0xCC7A56);
    Test.assertEqual(Snap.pctColor(85, 0xCC7A56), 0xC24B3A);
    return true;
}
