package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dinglebop/claude-pulse/relay/internal/activity"
	"github.com/dinglebop/claude-pulse/relay/internal/anthropic"
	"github.com/dinglebop/claude-pulse/relay/internal/config"
	"github.com/dinglebop/claude-pulse/relay/internal/server"
	"github.com/dinglebop/claude-pulse/relay/internal/store"
)

func main() {
	home, _ := os.UserHomeDir()
	listen := flag.String("listen", "", "listen address (overrides config)")
	noTunnel := flag.Bool("no-tunnel", false, "do not spawn cloudflared quick tunnel")
	jobsDir := flag.String("jobs-dir", filepath.Join(home, ".claude", "jobs"), "Claude jobs dir")
	base := flag.String("anthropic-base", "https://api.anthropic.com", "Anthropic API base URL")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	poller := anthropic.NewUsagePoller(*base, func() (anthropic.Credentials, error) {
		return anthropic.LoadCredentials()
	})
	go func() {
		for {
			poller.Poll(time.Now())
			time.Sleep(30 * time.Second) // Poll() self-gates to >=5 min
		}
	}()

	h := server.New(cfg.Token, st, server.Providers{
		Usage:    poller.Current,
		Activity: func() (bool, int) { return activity.Check(*jobsDir) },
		Daily: func() ([]store.DayTotal, error) {
			return st.Daily(time.Now().UTC().Format("2006-01-02"), 7)
		},
	})

	if !*noTunnel {
		log.Println("tunnel: not yet implemented, run with --no-tunnel or wait for Task 10")
	}
	log.Printf("listening on %s", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, h))
}
