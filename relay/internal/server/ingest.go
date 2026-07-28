package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/dinglebop/claude-pulse/relay/internal/store"
)

const seenTTL = 24 * time.Hour

type sessionSeen struct {
	cost   float64
	tokens int64
	last   time.Time
}

func IngestHandler(st *store.Store, today func() string, now func() time.Time) http.Handler {
	var mu sync.Mutex
	seen := map[string]sessionSeen{}
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
		mu.Lock()
		prev := seen[p.SessionID]
		dCost := p.Cost.TotalCostUSD - prev.cost
		dTok := total - prev.tokens
		if dCost < 0 {
			dCost = p.Cost.TotalCostUSD // session restarted; count fresh
		}
		if dTok < 0 {
			dTok = total
		}
		// Hold lock across AddCost to ensure seen is only advanced on success.
		// Contention is negligible under single-user local daemon load.
		if err := st.AddCost(today(), dCost, dTok); err != nil {
			mu.Unlock()
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		t := now()
		seen[p.SessionID] = sessionSeen{cost: p.Cost.TotalCostUSD, tokens: total, last: t}
		for id, s := range seen {
			if t.Sub(s.last) > seenTTL {
				delete(seen, id)
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}
