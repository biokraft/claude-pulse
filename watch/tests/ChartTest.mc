import Toybox.Test;
import Toybox.Lang;

(:test)
function testBarHeights(logger as Test.Logger) as Boolean {
    var daily = [
        {"day" => "d1", "cost_usd" => 1.0, "tokens" => 100},
        {"day" => "d2", "cost_usd" => 2.0, "tokens" => 100},
        {"day" => "d3", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d4", "cost_usd" => 0.01, "tokens" => 10},
        {"day" => "d5", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d6", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d7", "cost_usd" => 0.0, "tokens" => 0}
    ];
    var heights = Chart.barHeights(daily, 56);
    Test.assertEqual(heights[1], 56); // max maps to maxPx
    Test.assertEqual(heights[2], 0);  // zero -> 0
    Test.assertEqual(heights[4], 0);
    Test.assert(heights[3] >= 2); // small nonzero value still visible
    return true;
}

(:test)
function testBarHeightsAllZero(logger as Test.Logger) as Boolean {
    var daily = [
        {"day" => "d1", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d2", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d3", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d4", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d5", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d6", "cost_usd" => 0.0, "tokens" => 0},
        {"day" => "d7", "cost_usd" => 0.0, "tokens" => 0}
    ];
    var heights = Chart.barHeights(daily, 56);
    for (var i = 0; i < heights.size(); i += 1) {
        Test.assertEqual(heights[i], 0);
    }
    return true;
}

(:test)
function testFormatTokens(logger as Test.Logger) as Boolean {
    Test.assertEqual(Chart.formatTokens(2100000), "2.1M");
    Test.assertEqual(Chart.formatTokens(812000), "812K");
    Test.assertEqual(Chart.formatTokens(950), "950");
    return true;
}
