package anthropic

import (
	"net/http"
	"net/http/httptest"
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
