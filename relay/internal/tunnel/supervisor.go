package tunnel

import (
	"context"
	"net/http"
	"time"
)

// ProbeFunc checks whether the tunnel reachable at url is currently healthy.
type ProbeFunc func(ctx context.Context, url string) bool

// RestartFunc tears down the current tunnel and brings up a replacement,
// returning its (possibly new) public URL.
type RestartFunc func(ctx context.Context) (string, error)

const (
	defaultFailuresBeforeRestart = 2
	defaultInterval              = 60 * time.Second
)

// Supervisor watches a tunnel's liveness and restarts it after it has been
// unreachable for a run of consecutive probes. It has no network code of its
// own — Probe and Restart are injected so the state machine unit-tests
// without cloudflared or a real HTTP server.
type Supervisor struct {
	Probe                 ProbeFunc
	Restart               RestartFunc
	Logf                  func(string, ...any)
	Interval              time.Duration
	FailuresBeforeRestart int

	// Ticks lets tests drive the loop deterministically instead of waiting on
	// a real ticker. When nil, Run builds one from Interval.
	Ticks <-chan time.Time

	// cycled, when non-nil, receives once after each completed tick. Tests use
	// it to observe a finished cycle instead of sleeping — a sleep is not a
	// happens-before edge, so assertions on state the run loop writes would
	// race.
	cycled chan<- struct{}
}

// Run probes currentURL on every tick until ctx is cancelled, restarting the
// tunnel once probes fail FailuresBeforeRestart times in a row. A single
// blip is tolerated on purpose: quick tunnels and the network between here
// and them both have transient hiccups that resolve on their own, and
// restarting on the first miss would rotate the public URL — and force a
// watch re-pair — far more often than necessary.
func (s *Supervisor) Run(ctx context.Context, initialURL string) {
	failuresBeforeRestart := s.FailuresBeforeRestart
	if failuresBeforeRestart == 0 {
		failuresBeforeRestart = defaultFailuresBeforeRestart
	}
	interval := s.Interval
	if interval == 0 {
		interval = defaultInterval
	}

	ticks := s.Ticks
	if ticks == nil {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		ticks = ticker.C
	}

	currentURL := initialURL
	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if s.Probe(ctx, currentURL) {
				consecutiveFailures = 0
			} else {
				consecutiveFailures++
				if consecutiveFailures >= failuresBeforeRestart {
					fresh, err := s.Restart(ctx)
					if err != nil {
						// Leave the counter at the threshold rather than
						// resetting it: the tunnel is still down, so the
						// very next tick must retry the restart, not go
						// back to tolerating failures.
						s.Logf("tunnel restart failed: %v", err)
					} else {
						currentURL = fresh
						consecutiveFailures = 0
					}
				}
			}

			// Signal that this tick's work — probe, and any restart attempt
			// and state update — is fully done. Tests block on this instead
			// of sleeping, which would not be a real happens-before edge.
			if s.cycled != nil {
				select {
				case s.cycled <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// HTTPProbe builds a ProbeFunc that hits this relay's own snapshot endpoint.
// It deliberately omits the access token: an unauthenticated request that
// reaches our handler still gets a 401, which is enough to prove the tunnel
// is alive, and never risks leaking the token to a stale or hijacked URL.
func HTTPProbe(client *http.Client) ProbeFunc {
	return func(ctx context.Context, url string) bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/v1/snapshot", nil)
		if err != nil {
			return false
		}
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		// 401 is this server's own signature for "reachable but
		// unauthenticated". Anything else — 200, 502, Cloudflare's 530 —
		// means either the tunnel is down or we're looking at an edge error
		// page instead of our handler.
		return resp.StatusCode == http.StatusUnauthorized
	}
}
