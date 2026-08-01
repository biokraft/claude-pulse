package store

import (
	"path/filepath"
	"testing"
)

func TestAddCostAccumulatesAndDailyZeroFills(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.AddCost("2026-07-27", 1.50, 1000)
	s.AddCost("2026-07-27", 0.25, 500)
	s.AddCost("2026-07-25", 3.00, 2000)
	got, err := s.Daily("2026-07-27", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 {
		t.Fatalf("len = %d", len(got))
	}
	last := got[6]
	if last.Day != "2026-07-27" || last.CostUSD != 1.75 || last.Tokens != 1500 {
		t.Fatalf("today = %+v", last)
	}
	if got[4].CostUSD != 3.00 { // 2026-07-25
		t.Fatalf("d-2 = %+v", got[4])
	}
	if got[0].CostUSD != 0 || got[0].Day != "2026-07-21" {
		t.Fatalf("oldest = %+v", got[0])
	}
}
