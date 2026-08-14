package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

type DayTotal struct {
	Day     string  `json:"day"`
	CostUSD float64 `json:"cost_usd"`
	Tokens  int64   `json:"tokens"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS daily (
		day TEXT PRIMARY KEY, cost_usd REAL NOT NULL DEFAULT 0, tokens INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		return nil, err
	}
	// The running totals each session has already been credited for. These
	// used to be an in-memory map, which meant a relay restart forgot them and
	// counted a long-running session's entire cumulative cost as one fresh
	// delta — inflating the day's total by tens of dollars, once per restart,
	// with nothing in the log to say so.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		cost_usd REAL NOT NULL DEFAULT 0,
		tokens INTEGER NOT NULL DEFAULT 0,
		last TEXT NOT NULL)`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Daily(endDay string, n int) ([]DayTotal, error) {
	end, err := time.Parse("2006-01-02", endDay)
	if err != nil {
		return nil, err
	}
	out := make([]DayTotal, 0, n)
	for i := n - 1; i >= 0; i-- {
		day := end.AddDate(0, 0, -i).Format("2006-01-02")
		row := s.db.QueryRow(`SELECT cost_usd, tokens FROM daily WHERE day = ?`, day)
		d := DayTotal{Day: day}
		if err := row.Scan(&d.CostUSD, &d.Tokens); err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// sessionTTL is how long a session's running total is remembered. Claude Code
// reuses a session id for as long as the conversation lives, so this only has
// to outlast a working day.
const sessionTTL = 24 * time.Hour

// RecordSession credits the day with whatever part of a session's cumulative
// totals has not been counted yet, and returns the deltas it applied.
//
// Statusline payloads carry cumulative per-session figures, so the delta must
// be computed against what was already credited. Doing that here, in one
// transaction against durable state, is what makes the arithmetic survive a
// restart.
func (s *Store) RecordSession(day, sessionID string, totalCost float64, totalTokens int64, now time.Time) (float64, int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit has succeeded

	var prevCost float64
	var prevTokens int64
	row := tx.QueryRow(`SELECT cost_usd, tokens FROM sessions WHERE session_id = ?`, sessionID)
	if err := row.Scan(&prevCost, &prevTokens); err != nil && err != sql.ErrNoRows {
		return 0, 0, err
	}

	dCost := totalCost - prevCost
	dTokens := totalTokens - prevTokens
	// A total that went backwards means the session was restarted and its
	// counter reset, so the new figure is itself the uncounted amount.
	if dCost < 0 {
		dCost = totalCost
	}
	if dTokens < 0 {
		dTokens = totalTokens
	}

	if _, err := tx.Exec(`INSERT INTO daily(day, cost_usd, tokens) VALUES(?,?,?)
		ON CONFLICT(day) DO UPDATE SET cost_usd = cost_usd + excluded.cost_usd,
		tokens = tokens + excluded.tokens`, day, dCost, dTokens); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`INSERT INTO sessions(session_id, cost_usd, tokens, last) VALUES(?,?,?,?)
		ON CONFLICT(session_id) DO UPDATE SET cost_usd = excluded.cost_usd,
		tokens = excluded.tokens, last = excluded.last`,
		sessionID, totalCost, totalTokens, now.UTC().Format(time.RFC3339)); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE last < ?`,
		now.Add(-sessionTTL).UTC().Format(time.RFC3339)); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return dCost, dTokens, nil
}
