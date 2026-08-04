import Toybox.Test;
import Toybox.Lang;

(:test)
function testSyncedLabelMinutes(logger as Test.Logger) as Boolean {
    Test.assertEqual(Snap.syncedLabel(0), "synced just now");
    Test.assertEqual(Snap.syncedLabel(1), "synced 1m ago");
    Test.assertEqual(Snap.syncedLabel(59), "synced 59m ago");
    return true;
}

(:test)
function testSyncedLabelRollsUpToHours(logger as Test.Logger) as Boolean {
    // The bug this replaces: 492 minutes rendered as "synced 492m ago".
    Test.assertEqual(Snap.syncedLabel(492), "synced 8h 12m ago");
    Test.assertEqual(Snap.syncedLabel(60), "synced 1h ago");
    Test.assertEqual(Snap.syncedLabel(1439), "synced 23h 59m ago");
    return true;
}

(:test)
function testSyncedLabelRollsUpToDays(logger as Test.Logger) as Boolean {
    Test.assertEqual(Snap.syncedLabel(1440), "synced 1d ago");
    Test.assertEqual(Snap.syncedLabel(1500), "synced 1d 1h ago");
    Test.assertEqual(Snap.syncedLabel(4500), "synced 3d 3h ago");
    return true;
}

(:test)
function testSyncedLabelNeverSynced(logger as Test.Logger) as Boolean {
    // ageMinutes returns -1 when nothing has ever been stored.
    Test.assertEqual(Snap.syncedLabel(-1), "never synced");
    return true;
}
