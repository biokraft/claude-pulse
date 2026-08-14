package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/biokraft/claude-pulse/relay/internal/activity"
	"github.com/biokraft/claude-pulse/relay/internal/anthropic"
	"github.com/biokraft/claude-pulse/relay/internal/config"
	"github.com/biokraft/claude-pulse/relay/internal/hook"
	"github.com/biokraft/claude-pulse/relay/internal/server"
	"github.com/biokraft/claude-pulse/relay/internal/service"
	"github.com/biokraft/claude-pulse/relay/internal/status"
	"github.com/biokraft/claude-pulse/relay/internal/store"
	"github.com/biokraft/claude-pulse/relay/internal/tunnel"
)

// version is overridden at build time with -ldflags "-X main.version=…".
var version = "dev"

const usageText = `claude-pulse-relay — feeds the Claude Pulse Garmin watch app with your
Claude Code usage. Your credentials never leave this machine.

Usage:
  claude-pulse-relay [flags]              start the relay (default)
  claude-pulse-relay service install      run it in the background, across reboots
  claude-pulse-relay service uninstall    stop and remove that service
  claude-pulse-relay hook install         forward session cost from Claude Code
  claude-pulse-relay status               check the relay, tunnel and usage data
  claude-pulse-relay status -url <url>    also check a front-end you run yourself
  claude-pulse-relay version              print the version
  claude-pulse-relay help                 print this help

Flags:
  -listen <addr>           listen address (default from config, 127.0.0.1:8787)
  -no-tunnel               skip the Cloudflare quick tunnel and serve locally
  -jobs-dir <path>         Claude jobs directory to watch for active sessions
  -anthropic-base <url>    Anthropic API base URL

Starting with no arguments generates ~/.claude-pulse/config.json on first run,
then prints the pairing URL, token and QR code to enter in
Garmin Connect -> Connect IQ apps -> Claude Pulse -> Settings.

Docs: https://github.com/biokraft/claude-pulse/tree/main/relay
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "service":
			runServiceCmd(os.Args[2:])
			return
		case "hook":
			runHookCmd(os.Args[2:])
			return
		case "status":
			runStatusCmd(os.Args[2:])
			return
		case "help", "-h", "--help":
			fmt.Print(usageText)
			return
		case "version", "-v", "--version":
			fmt.Printf("claude-pulse-relay %s\n", version)
			return
		}
		// Anything that is not a flag is a mistyped subcommand. Without this,
		// flag.Parse silently ignores it and the relay starts anyway.
		if !strings.HasPrefix(os.Args[1], "-") {
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usageText)
			os.Exit(2)
		}
	}

	home, _ := os.UserHomeDir()
	listen := flag.String("listen", "", "listen address (overrides config)")
	noTunnel := flag.Bool("no-tunnel", false, "do not spawn cloudflared quick tunnel")
	jobsDir := flag.String("jobs-dir", filepath.Join(home, ".claude", "jobs"), "Claude jobs dir")
	base := flag.String("anthropic-base", "https://api.anthropic.com", "Anthropic API base URL")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usageText) }
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

	poller := anthropic.NewUsagePoller(*base, func() (anthropic.Credentials, error) {
		return anthropic.LoadCredentials()
	})
	poller.Logf = log.Printf
	// Persist the poll schedule so restarts honour any backoff already earned;
	// without this an upgrade loop re-triggers Anthropic's rate limiting.
	poller.StateFile(filepath.Join(cfg.Dir, "poll-state.json"))
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

	// Record where we listen before the tunnel is even attempted. Starting one
	// takes up to 30 seconds, and a receipt left behind by a crashed
	// predecessor would otherwise be read as this relay's, pointing `status`
	// at an address that died with the old process.
	wantTunnel := !*noTunnel && !cfg.NoTunnel
	recordRuntime := func(url string) {
		status.RecordRuntime(cfg.Dir,
			status.Runtime{Listen: cfg.Listen, URL: url, Tunnel: wantTunnel})
	}
	recordRuntime("")

	// tn is read and reassigned from two goroutines once the supervisor is
	// running: the supervisor's Restart closure replaces it after cloudflared
	// dies, and the main goroutine's shutdown path stops whatever is current.
	// tnMu keeps those two accesses from racing, and guarantees shutdown stops
	// the latest tunnel rather than a stale one it captured before a restart.
	var tnMu sync.Mutex
	var tn *tunnel.Tunnel
	// The config setting matters for services, which run with no arguments and
	// so can never pass --no-tunnel.
	if wantTunnel {
		tn, err = tunnel.Start(cfg.Listen, cfg.Token, os.Stdout)
		if err != nil {
			log.Printf("tunnel disabled: %v", err)
			tn = nil
		}
	}
	// `status` runs as a separate process and can learn this URL no other way:
	// cloudflared mints a new one on every start.
	if tn != nil {
		recordRuntime(tn.URL)
	}

	var cancelSup context.CancelFunc
	if tn != nil {
		sup := &tunnel.Supervisor{
			Probe: tunnel.HTTPProbe(&http.Client{Timeout: 10 * time.Second}),
			Restart: func(ctx context.Context) (string, error) {
				tnMu.Lock()
				current := tn
				tnMu.Unlock()
				current.Stop()

				fresh, err := tunnel.Start(cfg.Listen, cfg.Token, os.Stdout)
				if err != nil {
					return "", err
				}

				tnMu.Lock()
				tn = fresh
				tnMu.Unlock()

				recordRuntime(fresh.URL)
				log.Printf("tunnel public URL changed to %s — update the watch's relay URL setting", fresh.URL)
				return fresh.URL, nil
			},
			Logf: log.Printf,
		}
		var supCtx context.Context
		supCtx, cancelSup = context.WithCancel(context.Background())
		go sup.Run(supCtx, tn.URL)
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: h}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	serveErrCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.Listen)
		serveErrCh <- srv.ListenAndServe()
	}()

	exitCode := 0
	select {
	case <-sigCh:
		log.Printf("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
			exitCode = 1
		}
	case err := <-serveErrCh:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
			exitCode = 1
		}
	}

	if cancelSup != nil {
		cancelSup()
	}
	tnMu.Lock()
	final := tn
	tnMu.Unlock()
	if final != nil {
		final.Stop()
	}
	// The recorded address stops answering the moment this process exits.
	status.ClearRuntime(cfg.Dir)
	st.Close()
	os.Exit(exitCode)
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
		pulseHome, err := config.Home()
		if err != nil {
			log.Fatal(err)
		}
		logPath := filepath.Join(pulseHome, "relay.log")

		// A service inherits none of this shell's environment, so the PATH it
		// will run with has to be baked in — otherwise cloudflared, which is
		// usually in a Homebrew prefix, is invisible and the relay starts with
		// no tunnel on localhost only.
		var required []string
		cfPath, cfErr := exec.LookPath("cloudflared")
		if cfErr == nil {
			required = append(required, cfPath)
		}
		envPath := service.EnvPath(os.Getenv("PATH"), required...)

		var content string
		if runtime.GOOS == "darwin" {
			content = service.PlistContent(exePath, logPath, envPath)
		} else {
			content = service.UnitContent(exePath, envPath)
		}

		// Best-effort stop of any existing instance so install is idempotent.
		if _, err := os.Stat(path); err == nil {
			var stop *exec.Cmd
			if runtime.GOOS == "darwin" {
				stop = exec.Command("launchctl", "unload", path)
			} else {
				stop = exec.Command("systemctl", "--user", "disable", "--now", "claude-pulse-relay")
			}
			stop.Run() // ignore errors: not loaded is fine
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
		if cfErr != nil {
			fmt.Printf("\nwarning: cloudflared is not installed, so the relay will start\n"+
				"  without a tunnel and listen on localhost only — the watch cannot reach\n"+
				"  it that way. Install it, then re-run '%s service install':\n", exePath)
			if runtime.GOOS == "darwin" {
				fmt.Println("    brew install cloudflared")
			} else {
				fmt.Println("    https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/")
			}
			fmt.Println()
		}
		if runtime.GOOS == "darwin" {
			fmt.Printf("pairing QR + URL will appear in: %s\n   view with: tail -f %s\n", logPath, logPath)
		} else {
			fmt.Println("pairing QR + URL: journalctl --user -u claude-pulse-relay -f")
		}
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

	if err := hook.Install(settingsPath, cfg.Listen, cfg.Token); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("installed statusline hook in %s\n", settingsPath)
}

func runStatusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	publicURL := fs.String("url", "", "public URL to check as the watch's route in "+
		"(for a Tailscale Funnel or any other front-end the relay did not start)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	svcPath, err := servicePath()
	if err != nil {
		// A status report is still worth printing without this one line.
		svcPath = ""
	}
	home, _ := os.UserHomeDir()
	settingsPath := ""
	if home != "" {
		settingsPath = filepath.Join(home, ".claude", "settings.json")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report := status.Gather(ctx, status.Options{
		Dir:          cfg.Dir,
		Listen:       cfg.Listen,
		Token:        cfg.Token,
		NoTunnel:     cfg.NoTunnel,
		ServicePath:  svcPath,
		SettingsPath: settingsPath,
		PublicURL:    *publicURL,
	})
	status.Render(os.Stdout, report)

	// Non-zero on any finding, so this can be the health check in a script.
	if len(report.Problems()) > 0 {
		os.Exit(1)
	}
}
