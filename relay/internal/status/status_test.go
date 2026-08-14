package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// relayStub answers the way the real relay does: 401 without a token, the
// snapshot with one.
func relayStub(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sekret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func addrOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestGatherReportsAHealthyRelay(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	srv := relayStub(t, `{"five_hour_pct":33,"seven_day_pct":4,"active_count":2,
		"today_cost_usd":14.82,"today_tokens":2100000,
		"fetched_at":"2026-08-14T11:58:00Z","stale":false}`)

	dir := t.TempDir()
	RecordTunnelURL(dir, srv.URL)

	r := Gather(context.Background(), Options{
		Dir: dir, Listen: addrOf(t, srv), Token: "sekret", Now: now,
	})

	if !r.LocalUp {
		t.Errorf("LocalUp = false, want true (err: %s)", r.LocalErr)
	}
	if !r.TunnelUp {
		t.Errorf("TunnelUp = false, want true (err: %s)", r.TunnelErr)
	}
	if r.Snap == nil {
		t.Fatalf("Snap = nil, want the decoded snapshot (err: %s)", r.SnapErr)
	}
	if r.Snap.FiveHourPct != 33 || r.Snap.TodayCostUSD != 14.82 {
		t.Errorf("Snap = %+v, want the stub's values", *r.Snap)
	}
	if got := r.Problems(); len(got) != 0 {
		t.Errorf("Problems() = %v, want none", got)
	}
}

// The whole point of the command: a dead relay must be reported as one finding
// with a clear cause, not as a cascade of downstream symptoms.
func TestGatherReportsADeadRelayOnce(t *testing.T) {
	r := Gather(context.Background(), Options{
		Dir: t.TempDir(), Listen: "127.0.0.1:1", Token: "sekret",
		Client: &http.Client{Timeout: time.Second},
	})

	if r.LocalUp {
		t.Fatal("LocalUp = true against a closed port")
	}
	problems := r.Problems()
	if len(problems) != 1 || !strings.Contains(problems[0], "not running") {
		t.Errorf("Problems() = %v, want exactly one 'not running' entry", problems)
	}
}

// A relay that is up but whose tunnel died is the failure that left the watch
// stuck for three days, and is invisible from the local port alone.
func TestGatherFlagsALiveRelayBehindADeadTunnel(t *testing.T) {
	srv := relayStub(t, `{"stale":false,"fetched_at":"2026-08-14T11:58:00Z"}`)
	dir := t.TempDir()
	RecordTunnelURL(dir, "http://127.0.0.1:1")

	r := Gather(context.Background(), Options{
		Dir: dir, Listen: addrOf(t, srv), Token: "sekret",
		Client: &http.Client{Timeout: time.Second},
	})

	if !r.LocalUp {
		t.Fatalf("LocalUp = false, want true (err: %s)", r.LocalErr)
	}
	if r.TunnelUp {
		t.Error("TunnelUp = true against a closed port")
	}
	if !hasSubstring(r.Problems(), "tunnel URL does not answer") {
		t.Errorf("Problems() = %v, want a dead-tunnel entry", r.Problems())
	}
}

// no_tunnel means the user fronts the relay themselves (a Tailscale Funnel, a
// reverse proxy). Reporting a missing quick tunnel as a problem there would be
// permanent false noise.
func TestNoTunnelConfigIsNotAProblem(t *testing.T) {
	srv := relayStub(t, `{"stale":false,"fetched_at":"2026-08-14T11:58:00Z"}`)

	r := Gather(context.Background(), Options{
		Dir: t.TempDir(), Listen: addrOf(t, srv), Token: "sekret", NoTunnel: true,
	})

	if hasSubstring(r.Problems(), "tunnel") {
		t.Errorf("Problems() = %v, want no tunnel complaint when no_tunnel is set", r.Problems())
	}
}

func TestGatherReadsTheBackoffSchedule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "poll-state.json"),
		`{"next_due":"2026-08-14T12:30:00Z","interval":600000000000}`)

	r := Gather(context.Background(), Options{
		Dir: dir, Listen: "127.0.0.1:1", Client: &http.Client{Timeout: time.Second},
	})

	if r.PollInterval != 10*time.Minute {
		t.Errorf("PollInterval = %s, want 10m", r.PollInterval)
	}
	if r.NextPoll.IsZero() {
		t.Error("NextPoll is zero, want the file's timestamp")
	}
}

func TestHookDetection(t *testing.T) {
	dir := t.TempDir()
	installed := filepath.Join(dir, "installed.json")
	writeFile(t, installed,
		`{"statusLine":{"type":"command","command":"sh -c 'curl .../ingest/statusline?token=x'"}}`)
	other := filepath.Join(dir, "other.json")
	writeFile(t, other, `{"statusLine":{"type":"command","command":"my-own-statusline"}}`)

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"ours", installed, true},
		{"someone else's statusline", other, false},
		{"no settings file", filepath.Join(dir, "missing.json"), false},
		{"no path given", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hookInstalled(tc.path); got != tc.want {
				t.Errorf("hookInstalled(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestTunnelURLReceiptRoundTrips(t *testing.T) {
	dir := t.TempDir()

	if got := ReadTunnelURL(dir); got != "" {
		t.Errorf("ReadTunnelURL on a fresh dir = %q, want empty", got)
	}
	RecordTunnelURL(dir, "https://x.trycloudflare.com")
	if got := ReadTunnelURL(dir); got != "https://x.trycloudflare.com" {
		t.Errorf("ReadTunnelURL = %q, want the recorded URL", got)
	}

	// Shutdown clears it: a URL that stopped resolving the moment cloudflared
	// exited must not be reported as the address to pair against.
	ClearTunnelURL(dir)
	if got := ReadTunnelURL(dir); got != "" {
		t.Errorf("ReadTunnelURL after Clear = %q, want empty", got)
	}
	// Clearing twice is what happens when the relay is killed after a clean
	// shutdown already ran.
	ClearTunnelURL(dir)
}

func hasSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A user fronting the relay themselves has no receipt to read, so -url is the
// only way they get a reachability check at all.
func TestPublicURLOverridesTheReceipt(t *testing.T) {
	srv := relayStub(t, `{"stale":false,"fetched_at":"2026-08-14T11:58:00Z"}`)
	dir := t.TempDir()
	RecordTunnelURL(dir, "http://127.0.0.1:1") // a stale receipt, deliberately dead

	r := Gather(context.Background(), Options{
		Dir: dir, Listen: addrOf(t, srv), Token: "sekret", NoTunnel: true,
		PublicURL: srv.URL + "/", Client: &http.Client{Timeout: time.Second},
	})

	if r.TunnelURL != srv.URL {
		t.Errorf("TunnelURL = %q, want the supplied URL with its trailing slash trimmed", r.TunnelURL)
	}
	if !r.TunnelUp || !r.TunnelGiven {
		t.Errorf("TunnelUp = %v, TunnelGiven = %v, want both true (err: %s)",
			r.TunnelUp, r.TunnelGiven, r.TunnelErr)
	}
	if len(r.Problems()) != 0 {
		t.Errorf("Problems() = %v, want none", r.Problems())
	}
}
