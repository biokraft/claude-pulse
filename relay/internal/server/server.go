package server

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/biokraft/claude-pulse/relay/internal/anthropic"
	"github.com/biokraft/claude-pulse/relay/internal/quota"
	"github.com/biokraft/claude-pulse/relay/internal/store"
)

type Providers struct {
	Usage    func(now time.Time) (anthropic.Usage, time.Time, bool)
	LastCost func() (time.Time, bool)
	// Quota receives the quota figures carried by statusline payloads. It may
	// be nil, in which case they are ignored.
	Quota    *quota.Store
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

// serveLog records who fetched the snapshot and when. Without it there is no
// way to tell a watch that cannot reach the relay from a watch that reaches it
// and displays nothing — the two look identical from here, and the only person
// who can see the watch is the one asking for help.
type serveLog struct {
	mu     sync.Mutex
	last   time.Time
	agent  string
	denied time.Time
}

// StatusUserAgent identifies the status command's own requests, which are
// excluded from the log. Counting them would make the report claim the watch
// had just fetched every single time it was run.
const StatusUserAgent = "claude-pulse-status"

func (l *serveLog) record(t time.Time, agent string, ok bool) {
	if agent == StatusUserAgent {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if ok {
		l.last, l.agent = t, agent
		return
	}
	l.denied = t
}

// Last reports when the snapshot was last served, and to what.
func (l *serveLog) Last() (time.Time, string, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last, l.agent, l.denied
}

// Watch is the shared record of snapshot fetches, exported so the status
// command can report it.
var Watch = &serveLog{}

func New(token string, st *store.Store, p Providers) http.Handler {
	mux := http.NewServeMux()
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authorized(r, token) {
				Watch.record(time.Now(), r.UserAgent(), false)
				// A browser landing here scanned a stale QR code. Answer in
				// HTML: http.Error sends text/plain with nosniff, which mobile
				// Safari turns into a file download instead of a page.
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusUnauthorized)
					if _, err := w.Write([]byte(unauthorizedHTML)); err != nil {
						log.Printf("unauthorized page: %v", err)
					}
					return
				}
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	// The QR code points here, so "/" has to be something a phone browser can
	// render. Without it the default mux answered with a plain-text 404 that
	// mobile Safari offered as a file download.
	mux.Handle("GET /", guard(PairHandler(token)))
	// Browsers request this unprompted; without a route it 401s and shows up
	// as a console error on the pairing page.
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("POST /ingest/statusline", guard(IngestHandler(st, p.Quota, func() string {
		return time.Now().UTC().Format("2006-01-02")
	}, time.Now)))
	mux.Handle("GET /api/v1/snapshot", guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Watch.record(time.Now(), r.UserAgent(), true)
		u, fetched, stale := p.Usage(time.Now())
		active, n := p.Activity()
		daily, err := p.Daily()
		if err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		costAt := ""
		if p.LastCost != nil {
			if t, ok := p.LastCost(); ok {
				costAt = t.UTC().Format(time.RFC3339)
			}
		}
		quotaSource := ""
		if p.Quota != nil {
			if r, ok := p.Quota.Current(); ok && !r.At.Before(fetched) {
				quotaSource = r.Source
			} else if ok || fetched.Year() > 2000 {
				quotaSource = quota.SourcePoll
			}
		}
		servedTime, agent, denied := Watch.Last()
		servedAt, servedAgent, deniedAt := "", agent, ""
		if !servedTime.IsZero() {
			servedAt = servedTime.UTC().Format(time.RFC3339)
		}
		if !denied.IsZero() {
			deniedAt = denied.UTC().Format(time.RFC3339)
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
			"cost_last_at":        costAt,
			"quota_source":        quotaSource,
			"served_last_at":      servedAt,
			"served_last_agent":   servedAgent,
			"denied_last_at":      deniedAt,
			"stale":               stale,
		}); err != nil {
			// Headers already sent; only option is to log.
			log.Printf("snapshot encode: %v", err)
		}
	})))
	return mux
}
