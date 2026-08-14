// Package status answers "why is my watch showing dashes?" from a second
// process, without the user having to know the relay's token, curl its
// snapshot endpoint by hand, or read JSON out of ~/.claude-pulse.
//
// Everything it reports is gathered from three places: the config directory on
// disk, an HTTP call to the local listen address, and an HTTP call to the
// public tunnel URL. That last one is the whole point — it exercises the same
// path the watch uses, so a green line here means the watch can reach the
// relay too.
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Snapshot is the subset of the relay's /api/v1/snapshot payload worth
// reporting. It is deliberately not the whole thing: the point is to explain
// the relay's health, not to dump its data.
type Snapshot struct {
	FiveHourPct  float64 `json:"five_hour_pct"`
	SevenDayPct  float64 `json:"seven_day_pct"`
	ActiveCount  int     `json:"active_count"`
	TodayCostUSD float64 `json:"today_cost_usd"`
	TodayTokens  int64   `json:"today_tokens"`
	FetchedAt    string  `json:"fetched_at"`
	CostLastAt   string  `json:"cost_last_at"`
	Stale        bool    `json:"stale"`
}

// Report is everything `claude-pulse-relay status` knows. Render turns it into
// text; Problems turns it into the reason for a non-zero exit code.
type Report struct {
	Now time.Time

	ConfigPath string
	// Listen is where the relay actually is, which is the recorded address
	// when one was recorded and the configured one otherwise. ListenOverride
	// says the two disagreed, i.e. the relay was started with -listen.
	Listen         string
	ListenOverride bool
	TokenLen       int
	NoTunnel       bool

	// LocalUp is whether anything answered on Listen. LocalErr explains a
	// false, in the words of the transport rather than of a guess.
	LocalUp  bool
	LocalErr string

	// TunnelURL is the receipt written by the running relay, or the address
	// the user supplied; empty means no route to the watch is known.
	// TunnelUp is the result of actually fetching it, and TunnelGiven says
	// the address came from the user rather than from a tunnel we started.
	TunnelURL   string
	TunnelGiven bool
	TunnelUp    bool
	TunnelErr   string

	// Snap is nil when the snapshot could not be fetched or parsed —
	// SnapErr says which.
	Snap    *Snapshot
	SnapErr string

	// NextPoll and PollInterval come from poll-state.json. A NextPoll far in
	// the future is the signature of an Anthropic rate-limit backoff, which
	// is otherwise invisible.
	NextPoll     time.Time
	PollInterval time.Duration

	ServicePath      string
	ServiceInstalled bool
}

// Options are the inputs Gather needs. They are all explicit so tests can point
// the whole report at a temp directory and an httptest server.
type Options struct {
	Dir string
	// PublicURL overrides the URL to probe as the watch's route in. It is how
	// a user who fronts the relay themselves (a Tailscale Funnel, a reverse
	// proxy) gets the reachability check at all: there is no receipt to read,
	// because nothing the relay started owns that address.
	PublicURL   string
	Listen      string
	Token       string
	NoTunnel    bool
	ServicePath string
	Client      *http.Client
	Now         time.Time
}

// Gather collects the report. It never returns an error: an unreachable relay
// or an unreadable file is a finding to report, not a failure to run.
func Gather(ctx context.Context, o Options) Report {
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}

	// The receipt wins over the config: -listen is never written back, so a
	// relay started with an override lives somewhere config.json cannot name.
	rt, haveReceipt := ReadRuntime(o.Dir)
	listen := o.Listen
	if rt.Listen != "" {
		listen = rt.Listen
	}

	// What the running relay was told beats what the config says, for the same
	// reason as the listen address: --no-tunnel is a flag, not a setting.
	noTunnel := o.NoTunnel
	if haveReceipt {
		noTunnel = !rt.Tunnel
	}

	r := Report{
		Now:            now,
		ConfigPath:     filepath.Join(o.Dir, "config.json"),
		Listen:         listen,
		ListenOverride: rt.Listen != "" && rt.Listen != o.Listen,
		TokenLen:       len(o.Token),
		NoTunnel:       noTunnel,
		ServicePath:    o.ServicePath,
	}

	// The local check is unauthenticated on purpose: a 401 proves the relay's
	// own handler answered, which is all this line claims. Sending the token
	// here would prove the same thing while giving a wrong URL somewhere to
	// leak it to.
	r.LocalUp, r.LocalErr = probe(ctx, client, "http://"+listen)

	switch {
	case o.PublicURL != "":
		r.TunnelURL = strings.TrimSuffix(o.PublicURL, "/")
		r.TunnelGiven = true
	case !noTunnel:
		r.TunnelURL = rt.URL
	}
	if r.TunnelURL != "" {
		r.TunnelUp, r.TunnelErr = probe(ctx, client, r.TunnelURL)
	}

	if r.LocalUp {
		r.Snap, r.SnapErr = fetchSnapshot(ctx, client, "http://"+listen, o.Token)
	}

	r.NextPoll, r.PollInterval = readPollState(filepath.Join(o.Dir, "poll-state.json"))

	if o.ServicePath != "" {
		if _, err := os.Stat(o.ServicePath); err == nil {
			r.ServiceInstalled = true
		}
	}
	return r
}

// probe reports whether the relay's handler answered at base. A 401 counts as
// up: the request carried no token, so being rejected is the correct response
// and proves the relay — not a captive portal or a stale tunnel edge — is on
// the other end.
func probe(ctx context.Context, client *http.Client, base string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/snapshot", nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return true, ""
	}
	return false, fmt.Sprintf("answered HTTP %d, expected 401", resp.StatusCode)
}

func fetchSnapshot(ctx context.Context, client *http.Client, base, token string) (*Snapshot, string) {
	if token == "" {
		return nil, "no token in config"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/snapshot", nil)
	if err != nil {
		return nil, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	var s Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err.Error()
	}
	return &s, ""
}

func readPollState(path string) (time.Time, time.Duration) {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, 0
	}
	var s struct {
		NextDue  time.Time     `json:"next_due"`
		Interval time.Duration `json:"interval"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return time.Time{}, 0
	}
	return s.NextDue, s.Interval
}

// Problems lists what is wrong, in the order a user should fix it. An empty
// slice means healthy, and is what makes `status` usable as a check in a
// script.
func (r Report) Problems() []string {
	var out []string
	if !r.LocalUp {
		out = append(out, "the relay is not running on "+r.Listen)
		// Everything below depends on the relay answering, so reporting it
		// too would just be noise repeating the same root cause.
		return out
	}
	if r.TokenLen == 0 {
		out = append(out, "no token in "+r.ConfigPath)
	}
	if r.TunnelURL == "" && !r.NoTunnel {
		out = append(out, "no tunnel is running, so the watch cannot reach the relay")
	}
	if r.TunnelURL != "" && !r.TunnelUp {
		out = append(out, "the tunnel URL does not answer: "+r.TunnelErr)
	}
	// A missing token already appears above; reporting the snapshot it makes
	// unreadable would be the same root cause stated twice.
	if r.Snap == nil && r.SnapErr != "" && r.TokenLen > 0 {
		out = append(out, "cannot read the relay's snapshot: "+r.SnapErr)
	}
	if r.Snap != nil && r.Snap.Stale {
		out = append(out, "usage data is stale — the relay has not reached Anthropic recently")
	}
	return out
}
