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
	RecordRuntime(dir, Runtime{Listen: addrOf(t, srv), URL: srv.URL, Tunnel: true})

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
	RecordRuntime(dir, Runtime{Listen: addrOf(t, srv), URL: "http://127.0.0.1:1", Tunnel: true})

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

func TestRuntimeReceiptRoundTrips(t *testing.T) {
	dir := t.TempDir()

	if _, ok := ReadRuntime(dir); ok {
		t.Error("ReadRuntime on a fresh dir reported a receipt")
	}

	want := Runtime{Listen: "127.0.0.1:8799", URL: "https://x.trycloudflare.com", Tunnel: true}
	RecordRuntime(dir, want)
	got, ok := ReadRuntime(dir)
	if !ok || got != want {
		t.Errorf("ReadRuntime = %+v (ok=%v), want %+v", got, ok, want)
	}

	// A relay with no tunnel still records where it listens — that is how
	// `status` finds a -listen override at all.
	RecordRuntime(dir, Runtime{Listen: "127.0.0.1:9000"})
	if got, _ := ReadRuntime(dir); got.Listen != "127.0.0.1:9000" || got.URL != "" {
		t.Errorf("ReadRuntime = %+v, want the listen address and no URL", got)
	}

	// Shutdown clears it: an address that stopped answering the moment the
	// relay exited must not be reported as live.
	ClearRuntime(dir)
	if _, ok := ReadRuntime(dir); ok {
		t.Error("ReadRuntime after Clear reported a receipt")
	}
	// Clearing twice is what happens when a relay is killed after a clean
	// shutdown already ran.
	ClearRuntime(dir)
}

// A garbled receipt — a truncated write, a hand-edit — must not take the whole
// report down with it.
func TestUnreadableReceiptFallsBackToTheConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, runtimeFile), "{not json")

	r := Gather(context.Background(), Options{
		Dir: dir, Listen: "127.0.0.1:1", Client: &http.Client{Timeout: time.Second},
	})

	if r.Listen != "127.0.0.1:1" {
		t.Errorf("Listen = %q, want the configured address", r.Listen)
	}
	if r.ListenOverride {
		t.Error("ListenOverride = true from an unreadable receipt")
	}
}

// -listen is never written back to the config, so a report that trusted the
// config would probe the wrong port — and if another relay happened to be
// there, describe the wrong process entirely.
func TestRecordedListenBeatsTheConfig(t *testing.T) {
	srv := relayStub(t, `{"stale":false,"fetched_at":"2026-08-14T11:58:00Z"}`)
	dir := t.TempDir()
	RecordRuntime(dir, Runtime{Listen: addrOf(t, srv)})

	r := Gather(context.Background(), Options{
		Dir: dir, Listen: "127.0.0.1:1", Token: "sekret", NoTunnel: true,
		Client: &http.Client{Timeout: time.Second},
	})

	if r.Listen != addrOf(t, srv) {
		t.Errorf("Listen = %q, want the recorded address", r.Listen)
	}
	if !r.ListenOverride {
		t.Error("ListenOverride = false, want the disagreement reported")
	}
	if !r.LocalUp || r.Snap == nil {
		t.Errorf("probed the wrong address: LocalUp=%v SnapErr=%s", r.LocalUp, r.SnapErr)
	}
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
	RecordRuntime(dir, Runtime{Listen: addrOf(t, srv), URL: "http://127.0.0.1:1", Tunnel: true}) // stale URL, deliberately dead

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

// A missing token makes the snapshot unreadable by definition. Reporting both
// states one root cause twice and buries the actionable half.
func TestMissingTokenIsReportedOnce(t *testing.T) {
	srv := relayStub(t, `{}`)

	r := Gather(context.Background(), Options{
		Dir: t.TempDir(), Listen: addrOf(t, srv), Token: "", NoTunnel: true,
	})

	problems := r.Problems()
	if len(problems) != 1 || !strings.Contains(problems[0], "no token") {
		t.Errorf("Problems() = %v, want exactly one 'no token' entry", problems)
	}
}

// --no-tunnel is a flag, never written back to the config, so a relay started
// with it would otherwise be reported as missing a tunnel it was told not to
// open.
func TestRecordedTunnelIntentBeatsTheConfig(t *testing.T) {
	srv := relayStub(t, `{"stale":false,"fetched_at":"2026-08-14T11:58:00Z"}`)
	dir := t.TempDir()
	RecordRuntime(dir, Runtime{Listen: addrOf(t, srv), Tunnel: false})

	// The config says a tunnel is wanted; the running relay says otherwise.
	r := Gather(context.Background(), Options{
		Dir: dir, Listen: addrOf(t, srv), Token: "sekret", NoTunnel: false,
		Client: &http.Client{Timeout: time.Second},
	})

	if hasSubstring(r.Problems(), "tunnel") {
		t.Errorf("Problems() = %v, want no tunnel complaint", r.Problems())
	}
}
