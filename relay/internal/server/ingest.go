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
			ContextWindow struct {
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
			} `json:"context_window"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.SessionID == "" {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}

		total := p.ContextWindow.InputTokens + p.ContextWindow.OutputTokens
		if _, _, err := st.RecordSession(today(), p.SessionID,
			p.Cost.TotalCostUSD, total, now()); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
