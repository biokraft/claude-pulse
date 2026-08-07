package anthropic

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	baseInterval = 5 * time.Minute
	maxInterval  = 60 * time.Minute
	staleAfter   = 15 * time.Minute
)

type Usage struct {
	FiveHourPct      float64
	SevenDayPct      float64
	FiveHourResetsAt time.Time
	SevenDayResetsAt time.Time
}

type UsagePoller struct {
	mu       sync.Mutex
	baseURL  string
	creds    func() (Credentials, error)
	client   *http.Client
	usage    Usage
	fetched  time.Time
	nextDue  time.Time
	interval time.Duration
	hasData  bool
	polling  bool

	// statePath persists the poll schedule across restarts; see pollstate.go.
	statePath string

	// Logf reports why a poll failed. Every failure path here used to return
	// silently, so a relay with expired credentials or a changed endpoint
	// looked identical to a healthy one: the watch just showed "--" forever
	// and the log said nothing. Never log the access token.
	Logf func(format string, args ...any)
}

func (p *UsagePoller) logf(format string, args ...any) {
	if p.Logf != nil {
		p.Logf(format, args...)
	}
}

func NewUsagePoller(baseURL string, creds func() (Credentials, error)) *UsagePoller {
	return &UsagePoller{baseURL: baseURL, creds: creds,
		client: &http.Client{Timeout: 30 * time.Second}, interval: baseInterval}
}

func (p *UsagePoller) Poll(now time.Time) {
	// Check if due and acquire polling state under lock.
	p.mu.Lock()
	if now.Before(p.nextDue) || p.polling {
		p.mu.Unlock()
		return
	}
	p.polling = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.polling = false
		p.mu.Unlock()
	}()

	// Fetch credentials and make HTTP request (unlocked).
	c, err := p.creds()
	if err != nil {
		p.logf("usage poll: cannot read Claude Code credentials: %v "+
			"(log in with Claude Code, then restart the relay)", err)
		p.mu.Lock()
		p.interval = baseInterval
		p.nextDue = now.Add(baseInterval)
		p.saveState()
		p.mu.Unlock()
		return
	}
	req, _ := http.NewRequest("GET", p.baseURL+"/api/oauth/usage", nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	resp, err := p.client.Do(req)
	if err != nil {
		p.logf("usage poll: request to Anthropic failed: %v", err)
		p.mu.Lock()
		p.interval = baseInterval
		p.nextDue = now.Add(baseInterval)
		p.saveState()
		p.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	// Decode response (unlocked).
	if resp.StatusCode == http.StatusTooManyRequests {
		p.logf("usage poll: rate limited by Anthropic, backing off")
		p.mu.Lock()
		p.interval *= 2
		if p.interval > maxInterval {
			p.interval = maxInterval
		}
		p.nextDue = now.Add(p.interval)
		p.saveState()
		p.mu.Unlock()
		return
	}
	if resp.StatusCode != http.StatusOK {
		hint := ""
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			hint = " (credentials look expired — log in with Claude Code again)"
		}
		p.logf("usage poll: Anthropic returned HTTP %d%s", resp.StatusCode, hint)
		p.mu.Lock()
		p.interval = baseInterval
		p.nextDue = now.Add(baseInterval)
		p.saveState()
		p.mu.Unlock()
		return
	}

	var raw struct {
		FiveHour struct {
			Utilization float64   `json:"utilization"`
			ResetsAt    time.Time `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization float64   `json:"utilization"`
			ResetsAt    time.Time `json:"resets_at"`
		} `json:"seven_day"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		p.logf("usage poll: could not decode Anthropic's response: %v "+
			"(the undocumented usage endpoint may have changed)", err)
		p.mu.Lock()
		p.interval = baseInterval
		p.nextDue = now.Add(baseInterval)
		p.saveState()
		p.mu.Unlock()
		return
	}

	// Update state under lock.
	p.mu.Lock()
	p.usage = Usage{
		FiveHourPct: raw.FiveHour.Utilization, SevenDayPct: raw.SevenDay.Utilization,
		FiveHourResetsAt: raw.FiveHour.ResetsAt, SevenDayResetsAt: raw.SevenDay.ResetsAt,
	}
	p.fetched = now
	p.hasData = true
	p.interval = baseInterval
	p.nextDue = now.Add(baseInterval)
	p.saveState()
	p.mu.Unlock()
}

func (p *UsagePoller) Current(now time.Time) (Usage, time.Time, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stale := !p.hasData || now.Sub(p.fetched) > staleAfter
	return p.usage, p.fetched, stale
}
