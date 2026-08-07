package tunnel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// drive runs the supervisor against a manual tick channel and returns a
// function that advances it one tick, blocking until that tick's cycle has
// fully completed — deterministic, unlike a sleep-based guess.
func drive(t *testing.T, s *Supervisor, initialURL string) (tick func(), stop func()) {
	t.Helper()
	ch := make(chan time.Time)
	acked := make(chan struct{})
	s.Ticks = ch
	s.cycled = acked
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx, initialURL); close(done) }()
	return func() {
			ch <- time.Now()
			select {
			case <-acked:
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for tick cycle to complete")
			}
		},
		func() { cancel(); <-done }
}

func TestSupervisorIgnoresSingleFailure(t *testing.T) {
	restarts := 0
	s := &Supervisor{
		Probe:                 func(context.Context, string) bool { return false },
		Restart:               func(context.Context) (string, error) { restarts++; return "https://new.example", nil },
		Logf:                  func(string, ...any) {},
		FailuresBeforeRestart: 2,
	}
	tick, stop := drive(t, s, "https://old.example")
	defer stop()

	tick()
	if restarts != 0 {
		t.Fatalf("restarted after a single failure (%d) — one blip must not rotate the URL", restarts)
	}
}

func TestSupervisorRestartsOnSecondConsecutiveFailure(t *testing.T) {
	restarts := 0
	s := &Supervisor{
		Probe:                 func(context.Context, string) bool { return false },
		Restart:               func(context.Context) (string, error) { restarts++; return "https://new.example", nil },
		Logf:                  func(string, ...any) {},
		FailuresBeforeRestart: 2,
	}
	tick, stop := drive(t, s, "https://old.example")
	defer stop()

	tick()
	tick()
	if restarts != 1 {
		t.Fatalf("restarts = %d, want 1 after two consecutive failures", restarts)
	}
}

func TestSupervisorProbesNewURLAfterRestart(t *testing.T) {
	var probed []string
	s := &Supervisor{
		Probe:                 func(_ context.Context, url string) bool { probed = append(probed, url); return false },
		Restart:               func(context.Context) (string, error) { return "https://new.example", nil },
		Logf:                  func(string, ...any) {},
		FailuresBeforeRestart: 2,
	}
	tick, stop := drive(t, s, "https://old.example")
	defer stop()

	tick()
	tick() // triggers restart
	tick() // must now probe the replacement
	if last := probed[len(probed)-1]; last != "https://new.example" {
		t.Fatalf("probed %q after restart, want the new URL", last)
	}
}

func TestSupervisorSuccessResetsFailureCount(t *testing.T) {
	healthy := false
	restarts := 0
	s := &Supervisor{
		Probe:                 func(context.Context, string) bool { return healthy },
		Restart:               func(context.Context) (string, error) { restarts++; return "https://new.example", nil },
		Logf:                  func(string, ...any) {},
		FailuresBeforeRestart: 2,
	}
	tick, stop := drive(t, s, "https://old.example")
	defer stop()

	tick()          // fail 1
	healthy = true  //
	tick()          // recovered, counter must reset
	healthy = false //
	tick()          // fail 1 again, not 2
	if restarts != 0 {
		t.Fatalf("restarts = %d, want 0 — a success between failures must reset the counter", restarts)
	}
}

func TestSupervisorSurvivesFailedRestart(t *testing.T) {
	attempts := 0
	s := &Supervisor{
		Probe:                 func(context.Context, string) bool { return false },
		Restart:               func(context.Context) (string, error) { attempts++; return "", errors.New("cloudflared gone") },
		Logf:                  func(string, ...any) {},
		FailuresBeforeRestart: 2,
	}
	tick, stop := drive(t, s, "https://old.example")
	defer stop()

	tick()
	tick() // restart attempt 1 fails
	tick()
	tick() // must try again rather than give up
	if attempts < 2 {
		t.Fatalf("restart attempts = %d, want the supervisor to keep trying", attempts)
	}
}

// The probe must recognise this server specifically: a 401 proves the request
// reached our handler, not a Cloudflare error page.
func TestHTTPProbeAcceptsOnly401(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{http.StatusUnauthorized, true},
		{http.StatusOK, false},
		{http.StatusBadGateway, false},
		{530, false},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("token") != "" {
				t.Error("probe must never send the access token")
			}
			w.WriteHeader(tc.status)
		}))
		got := HTTPProbe(srv.Client())(context.Background(), srv.URL)
		srv.Close()
		if got != tc.want {
			t.Errorf("status %d: probe = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestHTTPProbeFailsOnUnreachableHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	if HTTPProbe(&http.Client{Timeout: time.Second})(context.Background(), url) {
		t.Error("probe reported a dead endpoint as healthy")
	}
}
