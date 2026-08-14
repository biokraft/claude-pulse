package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/biokraft/claude-pulse/relay/internal/store"
)

func TestIngestAccumulatesDeltas(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	h := IngestHandler(st, func() string { return "2026-07-27" }, func() time.Time { return time.Now() })

	post := func(body string) int {
		req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	// cumulative posts from one session: 1.00 then 1.50 -> stored total 1.50, not 2.50
	if c := post(`{"session_id":"s1","cost":{"total_cost_usd":1.00},"context_window":{"input_tokens":600,"output_tokens":400}}`); c != 204 {
		t.Fatalf("code %d", c)
	}
	post(`{"session_id":"s1","cost":{"total_cost_usd":1.50},"context_window":{"input_tokens":900,"output_tokens":600}}`)
	post(`{"session_id":"s2","cost":{"total_cost_usd":0.30},"context_window":{"input_tokens":100,"output_tokens":50}}`)
	got, _ := st.Daily("2026-07-27", 1)
	if got[0].CostUSD != 1.80 {
		t.Fatalf("cost = %v, want 1.80", got[0].CostUSD)
	}
	if got[0].Tokens != 1650 {
		t.Fatalf("tokens = %v, want 1650", got[0].Tokens)
	}
}

func TestIngestRejectsBadJSON(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	h := IngestHandler(st, func() string { return "2026-07-27" }, func() time.Time { return time.Now() })
	req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader("{"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("code %d", rr.Code)
	}
}

func TestIngestReturns500OnStoreFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "t.db")
	st, _ := store.Open(dbPath)
	h := IngestHandler(st, func() string { return "2026-07-27" }, func() time.Time { return time.Now() })
	st.Close()

	// Post against closed store should return 500
	req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader(`{"session_id":"s1","cost":{"total_cost_usd":1.00},"context_window":{"input_tokens":600,"output_tokens":400}}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 500 {
		t.Fatalf("code %d, want 500", rr.Code)
	}
}

func TestIngestPrunesIdleSessions(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	cur := time.Unix(1_000_000, 0)
	h := IngestHandler(st, func() string { return "2026-07-28" }, func() time.Time { return cur })

	post := func(sessionID string, cost float64) {
		body := fmt.Sprintf(`{"session_id":%q,"cost":{"total_cost_usd":%f},"context_window":{"input_tokens":10,"output_tokens":5}}`, sessionID, cost)
		req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d", w.Code)
		}
	}

	post("old-session", 1.00)
	cur = cur.Add(25 * time.Hour)
	post("new-session", 0.50) // triggers prune of old-session

	// old-session was pruned: same cumulative total counts fresh again
	// (delta = full amount, not zero).
	post("old-session", 1.00)

	got, err := st.Daily("2026-07-28", 1)
	if err != nil {
		t.Fatal(err)
	}
	// 1.00 + 0.50 + 1.00 (re-counted after prune) = 2.50
	if got[0].CostUSD != 2.50 {
		t.Fatalf("cost = %v, want 2.50 (old-session must have been pruned)", got[0].CostUSD)
	}
}

// The bug this guards: the already-counted totals used to live in a map in the
// handler, so a relay restart forgot them and re-counted a long-running
// session's entire cumulative cost as one delta.
func TestIngestDoesNotDoubleCountAfterARestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "t.db")
	day := func() string { return "2026-07-27" }
	clock := func() time.Time { return time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC) }

	post := func(h http.Handler, body string) {
		t.Helper()
		req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("code %d", rr.Code)
		}
	}
	const payload = `{"session_id":"long-lived","cost":{"total_cost_usd":79.73},
		"context_window":{"input_tokens":1000,"output_tokens":500}}`

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	post(IngestHandler(st, day, clock), payload)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	// The relay restarts; the same session posts its cumulative total again.
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	post(IngestHandler(st2, day, clock), payload)

	got, err := st2.Daily("2026-07-27", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].CostUSD != 79.73 {
		t.Errorf("cost = %v, want 79.73 counted exactly once", got[0].CostUSD)
	}
	if got[0].Tokens != 1500 {
		t.Errorf("tokens = %v, want 1500 counted exactly once", got[0].Tokens)
	}
}
