package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dinglebop/claude-pulse/relay/internal/activity"
	"github.com/dinglebop/claude-pulse/relay/internal/anthropic"
	"github.com/dinglebop/claude-pulse/relay/internal/config"
	"github.com/dinglebop/claude-pulse/relay/internal/hook"
	"github.com/dinglebop/claude-pulse/relay/internal/server"
	"github.com/dinglebop/claude-pulse/relay/internal/service"
	"github.com/dinglebop/claude-pulse/relay/internal/store"
	"github.com/dinglebop/claude-pulse/relay/internal/tunnel"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "service":
			runServiceCmd(os.Args[2:])
			return
		case "hook":
			runHookCmd(os.Args[2:])
			return
		}
	}

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
		tn, err := tunnel.Start(cfg.Listen, cfg.Token, os.Stdout)
		if err != nil {
			log.Printf("tunnel disabled: %v", err)
		} else {
			defer tn.Stop()
		}
	}
	log.Printf("listening on %s", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, h))
}

func servicePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", "com.claudepulse.relay.plist"), nil
	}
	return filepath.Join(home, ".config", "systemd", "user", "claude-pulse-relay.service"), nil
}

func runServiceCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: claude-pulse-relay service install|uninstall")
		os.Exit(1)
	}

	path, err := servicePath()
	if err != nil {
		log.Fatal(err)
	}

	switch args[0] {
	case "install":
		exePath, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Fatal(err)
		}
		var content string
		if runtime.GOOS == "darwin" {
			content = service.PlistContent(exePath)
		} else {
			content = service.UnitContent(exePath)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			log.Fatal(err)
		}
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("launchctl", "load", path)
		} else {
			cmd = exec.Command("systemctl", "--user", "enable", "--now", "claude-pulse-relay")
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("failed to load service: %v\n%s", err, out)
		}
		fmt.Printf("installed and started service: %s\n", path)
	case "uninstall":
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("launchctl", "unload", path)
		} else {
			cmd = exec.Command("systemctl", "--user", "disable", "--now", "claude-pulse-relay")
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("warning: failed to stop service: %v\n%s", err, out)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}
		fmt.Printf("uninstalled service: %s\n", path)
	default:
		fmt.Fprintln(os.Stderr, "usage: claude-pulse-relay service install|uninstall")
		os.Exit(1)
	}
}

func runHookCmd(args []string) {
	if len(args) < 1 || args[0] != "install" {
		fmt.Fprintln(os.Stderr, "usage: claude-pulse-relay hook install")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	if err := hook.Install(settingsPath, cfg.Token); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("installed statusline hook in %s\n", settingsPath)
}
