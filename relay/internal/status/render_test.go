package status

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func renderTo(r Report) string {
	var buf bytes.Buffer
	Render(&buf, r)
	return buf.String()
}

// `status` is what a user runs before asking for help, so its output is the
// most likely thing in this project to end up pasted into a public issue.
func TestRenderNeverPrintsTheToken(t *testing.T) {
	out := renderTo(Report{
		Listen: "127.0.0.1:8787", LocalUp: true, TokenLen: len("sekret-token-value"),
		TunnelURL: "https://x.trycloudflare.com", TunnelUp: true,
	})

	if strings.Contains(out, "sekret") {
		t.Errorf("token leaked into the report:\n%s", out)
	}
	if !strings.Contains(out, "18 characters, not shown") {
		t.Errorf("want the token reported as present but withheld, got:\n%s", out)
	}
}

// Before its first successful poll the relay reports the zero time, and
// printing that raw ("0001-01-01T00:00:00Z") read as a corrupt date.
func TestRenderExplainsTheZeroFetchTime(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	out := renderTo(Report{
		Now: now, LocalUp: true, TokenLen: 32,
		Snap: &Snapshot{FetchedAt: "0001-01-01T00:00:00Z", Stale: true},
	})

	if !strings.Contains(out, "never — no successful poll yet") {
		t.Errorf("want a plain-language never, got:\n%s", out)
	}
	if strings.Contains(out, "0001") {
		t.Errorf("printed the raw zero time:\n%s", out)
	}
}

// A backoff is the only explanation for a relay that is up, authenticated and
// still serving nothing — so it has to be visible.
func TestRenderShowsTheBackoff(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	out := renderTo(Report{
		Now: now, LocalUp: true, TokenLen: 32,
		NextPoll: now.Add(22 * time.Minute), PollInterval: 40 * time.Minute,
	})

	if !strings.Contains(out, "in 22m") {
		t.Errorf("want the time until the next poll, got:\n%s", out)
	}
	if !strings.Contains(out, "backing off") {
		t.Errorf("want the raised interval called out, got:\n%s", out)
	}
}

// The base interval is normal operation, not a condition to warn about.
func TestRenderStaysQuietAtTheBaseInterval(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	out := renderTo(Report{
		Now: now, LocalUp: true, TokenLen: 32,
		NextPoll: now.Add(3 * time.Minute), PollInterval: 5 * time.Minute,
	})

	if strings.Contains(out, "backing off") {
		t.Errorf("warned about the default 5m interval:\n%s", out)
	}
}

func TestRenderSummarises(t *testing.T) {
	healthy := renderTo(Report{
		Listen: "127.0.0.1:8787", LocalUp: true, TokenLen: 32,
		TunnelURL: "https://x.trycloudflare.com", TunnelUp: true,
		Snap: &Snapshot{FetchedAt: time.Now().UTC().Format(time.RFC3339)},
	})
	if !strings.Contains(healthy, "everything looks healthy") {
		t.Errorf("want a healthy summary, got:\n%s", healthy)
	}

	broken := renderTo(Report{Listen: "127.0.0.1:8787", TokenLen: 32})
	if !strings.Contains(broken, "Problems") || !strings.Contains(broken, "not running") {
		t.Errorf("want the problem listed, got:\n%s", broken)
	}
}

func TestShort(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{9 * time.Minute, "9m"},
		{92 * time.Minute, "1h 32m"},
		{75 * time.Hour, "3d 3h"},
	}
	for _, tc := range cases {
		if got := short(tc.in); got != tc.want {
			t.Errorf("short(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
