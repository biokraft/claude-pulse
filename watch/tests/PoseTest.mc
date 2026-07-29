import Toybox.Test;
import Toybox.Lang;

(:test)
function testPosePriority(logger as Test.Logger) as Boolean {
    var now = 5000;
    // celebrate beats everything
    Test.assertEqual(Pose.compute(99, 99, true, 3, 100, now + 10, now), :celebrate);
    // annoyed beats working
    Test.assertEqual(Pose.compute(85, 10, true, 12, 0, 0, now), :annoyed);
    Test.assertEqual(Pose.compute(10, 90, false, 12, 0, 0, now), :annoyed);
    // working
    Test.assertEqual(Pose.compute(10, 10, true, 12, 0, 0, now), :working);
    // sleeping: night + inactive >= 63
    Test.assertEqual(Pose.compute(10, 10, false, 3, 63, 0, now), :sleeping);
    // night idle stages
    Test.assertEqual(Pose.compute(10, 10, false, 3, 20, 0, now), :idleYawn);
    Test.assertEqual(Pose.compute(10, 10, false, 3, 45, 0, now), :idleDoze);
    Test.assertEqual(Pose.compute(10, 10, false, 3, 62, 0, now), :idleCollapse);
    // day idle: stages don't apply
    Test.assertEqual(Pose.compute(10, 10, false, 12, 62, 0, now), :idle);
    Test.assertEqual(Pose.compute(10, 10, false, 3, 5, 0, now), :idle);
    return true;
}
