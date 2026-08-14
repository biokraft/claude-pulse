package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDailyAccumulatesAndZeroFills(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seed := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	mustRecord(t, s, "2026-07-27", "a", 1.50, 1000, seed)
	mustRecord(t, s, "2026-07-27", "b", 0.25, 500, seed)
	mustRecord(t, s, "2026-07-25", "c", 3.00, 2000, seed)
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

func TestRecordSessionCreditsOnlyTheUncountedPart(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)

	if d, tok, err := st.RecordSession("2026-07-27", "s1", 1.00, 1000, now); err != nil ||
		d != 1.00 || tok != 1000 {
		t.Fatalf("first call = (%v, %v, %v), want (1, 1000, nil)", d, tok, err)
	}
	if d, tok, err := st.RecordSession("2026-07-27", "s1", 1.50, 1600, now); err != nil ||
		d != 0.50 || tok != 600 {
		t.Fatalf("second call = (%v, %v, %v), want (0.5, 600, nil)", d, tok, err)
	}

	// A total that went backwards means the session restarted and its counter
	// reset, so the whole new figure is uncounted.
	if d, _, err := st.RecordSession("2026-07-27", "s1", 0.20, 100, now); err != nil || d != 0.20 {
		t.Fatalf("after a reset = (%v, %v), want (0.2, nil)", d, err)
	}

	got, err := st.Daily("2026-07-27", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].CostUSD != 1.70 {
		t.Errorf("day total = %v, want 1.70", got[0].CostUSD)
	}
}

// Session rows are working state, not history: without pruning the table grows
// without bound on a relay that runs for months.
func TestRecordSessionForgetsOldSessions(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	day1 := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)

	if _, _, err := st.RecordSession("2026-07-27", "old", 5.00, 100, day1); err != nil {
		t.Fatal(err)
	}
	// Two days later a different session posts, which prunes the stale row.
	if _, _, err := st.RecordSession("2026-07-29", "new", 1.00, 10,
		day1.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("sessions rows = %d, want 1 (the stale one pruned)", n)
	}
}

func mustRecord(t *testing.T, s *Store, day, session string, cost float64, tokens int64, now time.Time) {
	t.Helper()
	if _, _, err := s.RecordSession(day, session, cost, tokens, now); err != nil {
		t.Fatal(err)
	}
}
