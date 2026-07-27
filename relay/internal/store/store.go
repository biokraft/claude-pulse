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
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) AddCost(day string, costUSD float64, tokens int64) error {
	_, err := s.db.Exec(`INSERT INTO daily(day, cost_usd, tokens) VALUES(?,?,?)
		ON CONFLICT(day) DO UPDATE SET cost_usd = cost_usd + excluded.cost_usd,
		tokens = tokens + excluded.tokens`, day, costUSD, tokens)
	return err
}

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
