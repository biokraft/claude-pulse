package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/dinglebop/claude-pulse/relay/internal/store"
)

type sessionSeen struct {
	cost   float64
	tokens int64
}

func IngestHandler(st *store.Store, today func() string) http.Handler {
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
		seen[p.SessionID] = sessionSeen{cost: p.Cost.TotalCostUSD, tokens: total}
		mu.Unlock()
		if err := st.AddCost(today(), dCost, dTok); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
