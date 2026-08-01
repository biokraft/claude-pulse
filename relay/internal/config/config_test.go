package config

import (
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
