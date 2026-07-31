package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/biokraft/claude-pulse/relay/internal/anthropic"
	"github.com/biokraft/claude-pulse/relay/internal/store"
)

func testHandler(t *testing.T) *httptest.Server {
	t.Helper()
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	t.Cleanup(func() { st.Close() })
	st.AddCost("2026-07-27", 14.82, 2100000)
	fetched := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	h := New("sekret", st, Providers{
		Usage: func(now time.Time) (anthropic.Usage, time.Time, bool) {
			return anthropic.Usage{FiveHourPct: 68, SevenDayPct: 42,
				FiveHourResetsAt: fetched.Add(2 * time.Hour),
				SevenDayResetsAt: fetched.Add(96 * time.Hour)}, fetched, false
		},
		Activity: func() (bool, int) { return true, 2 },
		Daily:    func() ([]store.DayTotal, error) { return st.Daily("2026-07-27", 7) },
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestSnapshotAuthRequired(t *testing.T) {
	srv := testHandler(t)
	resp, _ := srv.Client().Get(srv.URL + "/api/v1/snapshot")
	if resp.StatusCode != 401 {
		t.Fatalf("code %d, want 401", resp.StatusCode)
	}
	resp, _ = srv.Client().Get(srv.URL + "/api/v1/snapshot?token=wrong")
	if resp.StatusCode != 401 {
		t.Fatalf("code %d, want 401", resp.StatusCode)
	}
}

func TestSnapshotPayload(t *testing.T) {
	srv := testHandler(t)
	resp, _ := srv.Client().Get(srv.URL + "/api/v1/snapshot?token=sekret")
	if resp.StatusCode != 200 {
		t.Fatalf("code %d", resp.StatusCode)
	}
	var got map[string]any
	json.NewDecoder(resp.Body).Decode(&got)
	if got["five_hour_pct"].(float64) != 68 || got["seven_day_pct"].(float64) != 42 {
		t.Fatalf("pcts: %v", got)
	}
	if got["is_active"] != true || got["active_count"].(float64) != 2 {
		t.Fatalf("activity: %v", got)
	}
	if got["today_cost_usd"].(float64) != 14.82 {
		t.Fatalf("cost: %v", got["today_cost_usd"])
	}
	if got["stale"] != false {
		t.Fatal("stale wrong")
	}
	if len(got["daily"].([]any)) != 7 {
		t.Fatal("daily len")
	}
}

func TestEmptyTokenConfigRejects(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	t.Cleanup(func() { st.Close() })
	fetched := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	h := New("", st, Providers{
		Usage: func(now time.Time) (anthropic.Usage, time.Time, bool) {
			return anthropic.Usage{}, fetched, false
		},
		Activity: func() (bool, int) { return false, 0 },
		Daily:    func() ([]store.DayTotal, error) { return nil, nil },
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	resp, _ := srv.Client().Get(srv.URL + "/api/v1/snapshot")
	if resp.StatusCode != 401 {
		t.Fatalf("empty token config: code %d, want 401", resp.StatusCode)
	}
}

func TestBearerHeaderStrictParsing(t *testing.T) {
	srv := testHandler(t)
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/snapshot", nil)
	req.Header.Set("Authorization", "sekret")
	resp, _ := srv.Client().Do(req)
	if resp.StatusCode != 401 {
		t.Fatalf("non-Bearer header: code %d, want 401", resp.StatusCode)
	}
}
