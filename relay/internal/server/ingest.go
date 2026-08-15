package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/biokraft/claude-pulse/relay/internal/store"
)

// IngestHandler accepts Claude Code statusline payloads and credits the day
// with the part of each session's cumulative totals that has not been counted
// yet.
//
// The already-counted totals live in the database rather than in this handler.
// They were a map here until a restart proved what that costs: the map came
// back empty, the next payload from a session already hours old was treated as
// entirely new, and the day gained that session's whole cumulative cost a
// second time.
func IngestHandler(st *store.Store, today func() string, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			SessionID string `json:"session_id"`
			Cost      struct {
				TotalCostUSD float64 `json:"total_cost_usd"`
			} `json:"cost"`
			// The cumulative per-session counters. An earlier version of this
			// read context_window.input_tokens / .output_tokens, which do not
			// exist at that level — Claude Code nests the per-message figures
			// under current_usage and puts the session totals here. The decode
			// succeeded and silently produced zero, so the watch's token page
			// read "0 tokens" while cost, from a field whose name happened to
			// be right, worked fine.
			ContextWindow struct {
				TotalInputTokens  int64 `json:"total_input_tokens"`
				TotalOutputTokens int64 `json:"total_output_tokens"`
			} `json:"context_window"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.SessionID == "" {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}

		total := p.ContextWindow.TotalInputTokens + p.ContextWindow.TotalOutputTokens
		if _, _, err := st.RecordSession(today(), p.SessionID,
			p.Cost.TotalCostUSD, total, now()); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
