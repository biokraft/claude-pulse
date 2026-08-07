package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Token  string `json:"token"`
	Listen string `json:"listen"`
	// NoTunnel makes the relay skip its built-in quick tunnel, the same as the
	// --no-tunnel flag. It exists as config because an installed service runs
	// with no arguments: without it, a launchd or systemd relay always opens a
	// quick tunnel, which competes with a Tailscale Funnel or any other
	// front-end the user has put in place.
	NoTunnel bool   `json:"no_tunnel,omitempty"`
	DBPath   string `json:"-"`
	Dir      string `json:"-"`
}

// Home returns the claude-pulse home directory: $CLAUDE_PULSE_HOME if set,
// otherwise ~/.claude-pulse.
func Home() (string, error) {
	if d := os.Getenv("CLAUDE_PULSE_HOME"); d != "" {
		return d, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".claude-pulse"), nil
}

func Load() (*Config, error) {
	dir, err := Home()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	c := &Config{Listen: "127.0.0.1:8787"}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	if c.Token == "" {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return nil, err
		}
		c.Token = hex.EncodeToString(raw)
		b, _ := json.MarshalIndent(c, "", "  ")
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return nil, err
		}
	}
	c.Dir = dir
	c.DBPath = filepath.Join(dir, "relay.db")
	return c, nil
}
