package quota

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func reading(at time.Time, five float64, src string) Reading {
	return Reading{FiveHourPct: five, SevenDayPct: 10, At: at, Source: src}
}

func TestNewerReadingWins(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	if _, ok := s.Current(); ok {
		t.Error("a fresh store reported a reading")
	}
	if !s.Set(reading(t0, 35, SourcePoll)) {
		t.Fatal("first Set was rejected")
	}
	if !s.Set(reading(t0.Add(time.Minute), 36, SourceStatusline)) {
		t.Fatal("a newer reading was rejected")
	}

	got, ok := s.Current()
	if !ok || got.FiveHourPct != 36 || got.Source != SourceStatusline {
		t.Errorf("Current() = %+v, want the newer statusline reading", got)
	}
}

// A statusline payload can land while a poll started earlier is still in
// flight. Whichever arrives second, the older observation must not win.
func TestOlderReadingIsRejected(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	s.Set(reading(t0, 36, SourceStatusline))

	if s.Set(reading(t0.Add(-2*time.Minute), 12, SourcePoll)) {
		t.Error("an older reading was accepted")
	}
	if got, _ := s.Current(); got.FiveHourPct != 36 {
		t.Errorf("Current() = %v, want the newer reading kept", got.FiveHourPct)
	}
	// Equal timestamps are not newer either, or a duplicate delivery could
	// swap the source out from under an identical reading.
	if s.Set(reading(t0, 99, SourcePoll)) {
		t.Error("a reading with an equal timestamp was accepted")
	}
}

func TestReadingWithNoTimestampIsRejected(t *testing.T) {
	s := New()
	if s.Set(Reading{FiveHourPct: 50, Source: SourceStatusline}) {
		t.Error("a reading with no timestamp was accepted")
	}
}

// FreshSince is what stands the poll down; getting it wrong either keeps
// polling for nothing or lets the data go stale in silence.
func TestFreshSince(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	if s.FreshSince(t0.Add(-time.Hour)) {
		t.Error("an empty store reported itself fresh")
	}
	s.Set(reading(t0, 35, SourceStatusline))

	if !s.FreshSince(t0.Add(-time.Minute)) {
		t.Error("a reading newer than the cutoff was reported stale")
	}
	if s.FreshSince(t0.Add(time.Minute)) {
		t.Error("a reading older than the cutoff was reported fresh")
	}
}

func TestReadingSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quota.json")
	t0 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	first := New()
	first.StateFile(path)
	first.Set(Reading{
		FiveHourPct: 35, SevenDayPct: 37,
		FiveHourResetsAt: t0.Add(2 * time.Hour),
		At:               t0, Source: SourceStatusline,
	})

	second := New()
	second.StateFile(path)
	got, ok := second.Current()
	if !ok {
		t.Fatal("nothing restored")
	}
	if got.FiveHourPct != 35 || got.SevenDayPct != 37 || got.Source != SourceStatusline {
		t.Errorf("restored %+v, want the persisted reading", got)
	}
	if !got.At.Equal(t0) {
		t.Errorf("At = %v, want the original observation time %v", got.At, t0)
	}
	if !got.FiveHourResetsAt.Equal(t0.Add(2 * time.Hour)) {
		t.Errorf("FiveHourResetsAt = %v, want it preserved", got.FiveHourResetsAt)
	}
}

// A truncated or hand-edited file must not take the relay down with it.
func TestUnreadableStateIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quota.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := New()
	s.StateFile(path)
	if _, ok := s.Current(); ok {
		t.Error("a garbled file produced a reading")
	}
	// And the store still works afterwards.
	if !s.Set(reading(time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC), 35, SourcePoll)) {
		t.Error("the store stopped accepting readings")
	}
}

// Precedence decides which source the watch is actually reading. Backwards, it
// would serve a stale poll over a live statusline while both looked healthy.
func TestPickPrefersTheLaterObservation(t *testing.T) {
	t0 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	s := New()
	if _, ok := s.Pick(t0); ok {
		t.Error("an empty store won against a poll")
	}

	s.Set(reading(t0.Add(time.Minute), 36, SourceStatusline))
	if _, ok := s.Pick(t0); !ok {
		t.Error("a newer statusline reading lost to an older poll")
	}
	// A poll that ran more recently is the better answer, which is what keeps
	// the relay correct once Claude Code stops running.
	if _, ok := s.Pick(t0.Add(2 * time.Minute)); ok {
		t.Error("an older statusline reading beat a newer poll")
	}
}

func TestIsStale(t *testing.T) {
	t0 := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	if IsStale(t0, t0.Add(StaleAfter-time.Minute)) {
		t.Error("a reading inside the window was called stale")
	}
	if !IsStale(t0, t0.Add(StaleAfter+time.Minute)) {
		t.Error("a reading past the window was not called stale")
	}
}
