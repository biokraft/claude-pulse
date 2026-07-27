package server

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dinglebop/claude-pulse/relay/internal/store"
)

func TestIngestAccumulatesDeltas(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	h := IngestHandler(st, func() string { return "2026-07-27" })

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
	h := IngestHandler(st, func() string { return "2026-07-27" })
	req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader("{"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("code %d", rr.Code)
	}
}
