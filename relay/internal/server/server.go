package server

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dinglebop/claude-pulse/relay/internal/anthropic"
	"github.com/dinglebop/claude-pulse/relay/internal/store"
)

type Providers struct {
	Usage    func(now time.Time) (anthropic.Usage, time.Time, bool)
	Activity func() (bool, int)
	Daily    func() ([]store.DayTotal, error)
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	got := r.URL.Query().Get("token")
	if got == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			got = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func New(token string, st *store.Store, p Providers) http.Handler {
	mux := http.NewServeMux()
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authorized(r, token) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	mux.Handle("POST /ingest/statusline", guard(IngestHandler(st, func() string {
		return time.Now().UTC().Format("2006-01-02")
	})))
	mux.Handle("GET /api/v1/snapshot", guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, fetched, stale := p.Usage(time.Now())
		active, n := p.Activity()
		daily, err := p.Daily()
		if err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		today := store.DayTotal{}
		if len(daily) > 0 {
			today = daily[len(daily)-1]
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"five_hour_pct":       u.FiveHourPct,
			"seven_day_pct":       u.SevenDayPct,
			"five_hour_resets_at": u.FiveHourResetsAt.UTC().Format(time.RFC3339),
			"seven_day_resets_at": u.SevenDayResetsAt.UTC().Format(time.RFC3339),
			"is_active":           active,
			"active_count":        n,
			"today_cost_usd":      today.CostUSD,
			"today_tokens":        today.Tokens,
			"daily":               daily,
			"fetched_at":          fetched.UTC().Format(time.RFC3339),
			"stale":               stale,
		}); err != nil {
			// Headers already sent; only option is to log.
			log.Printf("snapshot encode: %v", err)
		}
	})))
	return mux
}
