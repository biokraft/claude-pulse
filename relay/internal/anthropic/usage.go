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
		p.mu.Lock()
		p.nextDue = now.Add(baseInterval)
		p.mu.Unlock()
		return
	}
	req, _ := http.NewRequest("GET", p.baseURL+"/api/oauth/usage", nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	resp, err := p.client.Do(req)
	if err != nil {
		p.mu.Lock()
		p.nextDue = now.Add(baseInterval)
		p.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	// Decode response (unlocked).
	if resp.StatusCode == http.StatusTooManyRequests {
		p.mu.Lock()
		p.interval *= 2
		if p.interval > maxInterval {
			p.interval = maxInterval
		}
		p.nextDue = now.Add(p.interval)
		p.mu.Unlock()
		return
	}
	if resp.StatusCode != http.StatusOK {
		p.mu.Lock()
		p.nextDue = now.Add(baseInterval)
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
		p.mu.Lock()
		p.nextDue = now.Add(baseInterval)
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
	p.mu.Unlock()
}

func (p *UsagePoller) Current(now time.Time) (Usage, time.Time, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stale := !p.hasData || now.Sub(p.fetched) > staleAfter
	return p.usage, p.fetched, stale
}
