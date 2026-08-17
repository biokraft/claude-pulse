// Package quota holds the most recent quota reading, whatever produced it.
//
// There are two sources. The relay polls Anthropic's usage endpoint, and
// Claude Code's statusline payload carries the same figures under rate_limits.
// The statusline is strictly better when it is flowing: it arrives the moment
// the numbers change rather than up to five minutes later, it costs no API
// call, and it cannot be rate limited — the rate limiting that repeatedly left
// this relay serving zeros was caused by polling.
//
// It is not a replacement, because it only arrives while Claude Code is
// running. Nothing is emitted overnight, so the poll remains the source of
// record when the statusline goes quiet. This package holds whichever reading
// is newer and remembers which one it was.
package quota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Source names where a reading came from, and is reported so the difference is
// visible rather than guessed at.
const (
	SourceStatusline = "statusline"
	SourcePoll       = "anthropic"
)

// Reading is one observation of the account's quota.
type Reading struct {
	FiveHourPct      float64   `json:"five_hour_pct"`
	SevenDayPct      float64   `json:"seven_day_pct"`
	FiveHourResetsAt time.Time `json:"five_hour_resets_at"`
	SevenDayResetsAt time.Time `json:"seven_day_resets_at"`
	At               time.Time `json:"at"`
	Source           string    `json:"source"`
}

// Store keeps the newest reading. It is safe for concurrent use: the statusline
// arrives on an HTTP handler goroutine while the poller runs on its own.
type Store struct {
	mu   sync.Mutex
	cur  Reading
	has  bool
	path string
}

// New returns an empty store.
func New() *Store { return &Store{} }

// StateFile makes the store persist its reading to path and loads whatever is
// already there. Errors are deliberately non-fatal: a relay that cannot read
// or write this file should still serve, just without carrying its last
// reading across a restart.
func (s *Store) StateFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = path

	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var r Reading
	if err := json.Unmarshal(b, &r); err != nil {
		return
	}
	if !r.At.IsZero() {
		s.cur, s.has = r, true
	}
}

// Set records a reading if it is newer than the one held. Out-of-order arrivals
// are real: a statusline payload can land while a poll started earlier is still
// in flight, and the older of the two must not win.
func (s *Store) Set(r Reading) bool {
	if r.At.IsZero() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.has && !r.At.After(s.cur.At) {
		return false
	}
	s.cur, s.has = r, true
	s.save()
	return true
}

// Current returns the held reading, and whether there is one.
func (s *Store) Current() (Reading, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur, s.has
}

// FreshSince reports whether the held reading is newer than cutoff. The poll
// loop uses it to skip an API call the statusline has already made redundant.
func (s *Store) FreshSince(cutoff time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.has && s.cur.At.After(cutoff)
}

// save persists the reading. The caller must hold s.mu.
func (s *Store) save() {
	if s.path == "" {
		return
	}
	b, err := json.Marshal(s.cur)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return
	}
	// Write via a temp file so a crash mid-write cannot leave an unparseable
	// reading behind.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
	}
}

// StaleAfter is how old a reading may be before the watch is told it is stale.
// It matches the poller's own threshold so the answer does not depend on which
// source happened to serve it.
const StaleAfter = 15 * time.Minute

// Pick returns this store's reading when it was observed later than the poll's,
// and false when the poll is the better answer.
//
// It is a method rather than an inline closure so the precedence can be tested:
// backwards, it would serve a stale poll over a live statusline while both
// sources looked healthy from the outside.
func (s *Store) Pick(polledAt time.Time) (Reading, bool) {
	r, ok := s.Current()
	if !ok || !r.At.After(polledAt) {
		return Reading{}, false
	}
	return r, true
}

// IsStale reports whether a reading taken at at is too old to present as
// current.
func IsStale(at time.Time, now time.Time) bool {
	return now.Sub(at) > StaleAfter
}
