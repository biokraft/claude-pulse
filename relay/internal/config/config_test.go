package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesConfigWithToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_PULSE_HOME", dir)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Token) != 32 {
		t.Fatalf("token len = %d, want 32", len(c.Token))
	}
	if c.Listen != "127.0.0.1:8787" {
		t.Fatalf("listen = %q", c.Listen)
	}
	if c.DBPath != filepath.Join(dir, "relay.db") {
		t.Fatalf("dbpath = %q", c.DBPath)
	}
}

func TestLoadIsStable(t *testing.T) {
	t.Setenv("CLAUDE_PULSE_HOME", t.TempDir())
	a, _ := Load()
	b, _ := Load()
	if a.Token != b.Token {
		t.Fatal("token changed between loads")
	}
}

// A service runs with no arguments, so --no-tunnel is unreachable for it. The
// config field is the only way to stop an installed service opening a quick
// tunnel that competes with a Funnel or reverse proxy.
func TestLoadReadsNoTunnel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_PULSE_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"token":"t","listen":"127.0.0.1:8787","no_tunnel":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.NoTunnel {
		t.Error("no_tunnel:true in config.json did not reach Config.NoTunnel")
	}
}

func TestLoadDefaultsToTunnelEnabled(t *testing.T) {
	t.Setenv("CLAUDE_PULSE_HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.NoTunnel {
		t.Error("a fresh config disabled the tunnel; the default must be enabled")
	}
}
