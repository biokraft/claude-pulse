package anthropic

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fixedCreds() (Credentials, error) {
	return Credentials{AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestPollSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"five_hour":{"utilization":68,"resets_at":"2026-07-27T12:00:00Z"},"seven_day":{"utilization":42,"resets_at":"2026-07-30T00:00:00Z"}}`))
	}))
	defer srv.Close()
	p := NewUsagePoller(srv.URL, fixedCreds)
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	p.Poll(now)
	u, fetched, stale := p.Current(now)
	if u.FiveHourPct != 68 || u.SevenDayPct != 42 {
		t.Fatalf("got %+v", u)
	}
	if stale || !fetched.Equal(now) {
		t.Fatalf("fetched=%v stale=%v", fetched, stale)
	}
}

func Test429BackoffServesStale(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Write([]byte(`{"five_hour":{"utilization":10,"resets_at":"2026-07-27T12:00:00Z"},"seven_day":{"utilization":5,"resets_at":"2026-07-30T00:00:00Z"}}`))
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	p := NewUsagePoller(srv.URL, fixedCreds)
	t0 := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	p.Poll(t0)                       // success, next due t0+5m
	p.Poll(t0.Add(5 * time.Minute))  // 429 -> wait 10m
	p.Poll(t0.Add(10 * time.Minute)) // not due, no call
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (backoff must skip)", calls)
	}
	p.Poll(t0.Add(16 * time.Minute)) // due again -> 429 -> wait 20m
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	u, _, stale := p.Current(t0.Add(16 * time.Minute))
	if u.FiveHourPct != 10 {
		t.Fatal("lost last-good data")
	}
	if !stale {
		t.Fatal("want stale=true after 16m without success")
	}
}

func TestCurrentWhilePollInFlight(t *testing.T) {
	// Test that Current() can be called and returns quickly while Poll
	// is blocked on a slow HTTP server.
	serverReady := make(chan struct{})
	serverDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(serverReady) // signal that request arrived
		<-serverDone       // wait for test to signal done
		w.Write([]byte(`{"five_hour":{"utilization":99,"resets_at":"2026-07-27T12:00:00Z"},"seven_day":{"utilization":88,"resets_at":"2026-07-30T00:00:00Z"}}`))
	}))
	defer srv.Close()

	p := NewUsagePoller(srv.URL, fixedCreds)
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)

	// Start Poll in background; it will block on the slow server.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Poll(now)
	}()

	// Wait for request to reach server.
	<-serverReady

	// Now call Current() while Poll is in flight.
	// Should return quickly without blocking on the HTTP request.
	done := make(chan struct{})
	go func() {
		u, _, stale := p.Current(now)
		// Initially no data.
		if u.FiveHourPct != 0 || u.SevenDayPct != 0 {
			t.Errorf("expected zero values, got %+v", u)
		}
		if !stale {
			t.Error("expected stale=true for uninitialized poller")
		}
		close(done)
	}()

	// If Current() was blocking on the mutex, this would timeout.
	select {
	case <-done:
		// Success: Current() returned quickly.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Current() blocked while Poll was in flight")
	}

	// Let server complete.
	close(serverDone)
	wg.Wait()

	// Verify Poll did complete successfully.
	u, _, _ := p.Current(now)
	if u.FiveHourPct != 99 || u.SevenDayPct != 88 {
		t.Fatalf("Poll did not complete; got %+v", u)
	}
}

func TestPollNon429ErrorResetsBackoff(t *testing.T) {
	// Server returns: 429, 500, 429. After the 500, escalation must
	// restart from baseInterval, so the final 429 yields 10 min, not 20.
	codes := []int{429, 500, 429}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(codes[i])
		i++
	}))
	defer srv.Close()

	p := NewUsagePoller(srv.URL, fixedCreds)
	t0 := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	p.Poll(t0)                       // 429 → interval 10m, nextDue t0+10m
	p.Poll(t0.Add(11 * time.Minute)) // 500 → interval resets to 5m
	t2 := t0.Add(17 * time.Minute)
	p.Poll(t2) // 429 → interval 10m (5m*2), NOT 20m

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.interval != 10*time.Minute {
		t.Fatalf("interval = %v, want 10m (escalation must restart after non-429 error)", p.interval)
	}
	if !p.nextDue.Equal(t2.Add(10 * time.Minute)) {
		t.Fatalf("nextDue = %v, want %v", p.nextDue, t2.Add(10*time.Minute))
	}
}

// Backoff lived only in memory, so restarting the relay polled Anthropic
// immediately every time. A few upgrades in a row was enough to earn a 429,
// and each restart renewed the ban instead of waiting it out.
func TestPollScheduleSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "poll-state.json")
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	creds := func() (Credentials, error) { return Credentials{AccessToken: "t"}, nil }

	first := NewUsagePoller(srv.URL, creds)
	first.StateFile(statePath)
	first.Poll(now) // gets 429 and backs off
	if calls != 1 {
		t.Fatalf("first poller made %d calls, want 1", calls)
	}

	// A fresh poller stands in for the relay being restarted.
	second := NewUsagePoller(srv.URL, creds)
	second.StateFile(statePath)
	second.Poll(now.Add(time.Minute))

	if calls != 1 {
		t.Fatalf("calls = %d — the restarted poller ignored the backoff and hit Anthropic again", calls)
	}
}

// A missing or corrupt state file must not stop the relay polling at all.
func TestPollStateToleratesBadFile(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "poll-state.json")
	if err := os.WriteFile(bad, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"five_hour":{"utilization":10},"seven_day":{"utilization":20}}`))
	}))
	defer srv.Close()

	p := NewUsagePoller(srv.URL, func() (Credentials, error) { return Credentials{AccessToken: "t"}, nil })
	p.StateFile(bad)
	p.Poll(time.Now())

	if calls != 1 {
		t.Fatalf("calls = %d, want 1 — a corrupt state file must not disable polling", calls)
	}
}

// A tampered file must not be able to make the relay poll harder than default.
func TestPollStateCannotShortenTheInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "poll-state.json")
	if err := os.WriteFile(path,
		[]byte(`{"next_due":"2000-01-01T00:00:00Z","interval":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewUsagePoller("http://127.0.0.1:1", func() (Credentials, error) { return Credentials{}, nil })
	p.StateFile(path)

	p.mu.Lock()
	got := p.interval
	p.mu.Unlock()
	if got < baseInterval {
		t.Errorf("interval = %v, want at least the %v default", got, baseInterval)
	}
}

// A relay that restarts while backed off cannot fetch anything for up to an
// hour. Without the last reading it serves zeros for that whole window, and the
// watch shows dashes for data the relay had already successfully fetched.
func TestLastReadingSurvivesRestart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`{"five_hour":{"utilization":68,"resets_at":"2026-07-27T12:00:00Z"},
			"seven_day":{"utilization":42,"resets_at":"2026-07-30T00:00:00Z"}}`)); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer srv.Close()
	statePath := filepath.Join(t.TempDir(), "poll-state.json")
	t0 := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)

	first := NewUsagePoller(srv.URL, fixedCreds)
	first.StateFile(statePath)
	first.Poll(t0)

	// A fresh poller, as after a restart, pointed at the same state file.
	second := NewUsagePoller(srv.URL, fixedCreds)
	second.StateFile(statePath)

	u, fetched, stale := second.Current(t0.Add(time.Minute))
	if u.FiveHourPct != 68 || u.SevenDayPct != 42 {
		t.Errorf("Usage = %+v, want the persisted reading", u)
	}
	if !fetched.Equal(t0) {
		t.Errorf("fetched = %v, want the original fetch time %v", fetched, t0)
	}
	if stale {
		t.Error("stale = true one minute after the persisted fetch")
	}
	if !u.FiveHourResetsAt.Equal(time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("FiveHourResetsAt = %v, want the persisted reset time", u.FiveHourResetsAt)
	}

	// The restored reading is not treated as fresh forever: it ages out on the
	// same clock as a live one, so the watch is never told stale data is new.
	if _, _, stale := second.Current(t0.Add(staleAfter + time.Minute)); !stale {
		t.Error("stale = false well past staleAfter")
	}
}

// Nothing has been fetched yet, so there is nothing to restore — and the
// absence must not be mistaken for a reading of zero.
func TestNoPersistedReadingLeavesTheRelayEmpty(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "poll-state.json")
	if err := os.WriteFile(statePath,
		[]byte(`{"next_due":"2026-07-27T11:00:00Z","interval":600000000000}`), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewUsagePoller("http://127.0.0.1:1", fixedCreds)
	p.StateFile(statePath)

	if _, _, stale := p.Current(time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)); !stale {
		t.Error("stale = false with no persisted reading")
	}
}
