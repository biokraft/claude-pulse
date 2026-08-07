package anthropic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Backoff used to live only in memory, so every relay restart reset it and
// polled Anthropic immediately. Upgrading the relay a few times in a row was
// therefore enough to earn an HTTP 429, and because the backoff reset again on
// the next restart, the ban kept being renewed rather than waited out. The
// watch just showed zeros.
//
// Persisting the schedule fixes that: a restarted relay honours the backoff its
// predecessor earned. The file holds no usage data and no credentials — only
// when the next poll is due — so it is not sensitive.
type pollState struct {
	NextDue  time.Time     `json:"next_due"`
	Interval time.Duration `json:"interval"`
}

// StateFile makes the poller persist its schedule to path. Errors are
// deliberately non-fatal: a relay that cannot write this file should still
// poll, just without the restart-proof backoff.
func (p *UsagePoller) StateFile(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statePath = path

	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s pollState
	if err := json.Unmarshal(b, &s); err != nil {
		return
	}
	if s.Interval >= baseInterval && s.Interval <= maxInterval {
		p.interval = s.Interval
	}
	// Only ever push the next poll further out. A stale or tampered file must
	// not be able to make the relay poll more aggressively than the default.
	if s.NextDue.After(p.nextDue) {
		p.nextDue = s.NextDue
	}
}

// saveState persists the schedule. The caller must hold p.mu.
func (p *UsagePoller) saveState() {
	if p.statePath == "" {
		return
	}
	b, err := json.Marshal(pollState{NextDue: p.nextDue, Interval: p.interval})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p.statePath), 0o700); err != nil {
		return
	}
	// Write via a temp file so a crash mid-write cannot leave the poller with
	// an unparseable schedule.
	tmp := p.statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, p.statePath); err != nil {
		os.Remove(tmp)
	}
}
