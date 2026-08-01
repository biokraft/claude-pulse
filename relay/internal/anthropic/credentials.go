package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Credentials struct {
	AccessToken string
	ExpiresAt   time.Time
}

func runSecurity(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func DefaultCredentialsPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".claude", ".credentials.json")
}

func parseCredentials(b []byte) (Credentials, error) {
	var raw struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Credentials{}, err
	}
	return Credentials{
		AccessToken: raw.ClaudeAiOauth.AccessToken,
		ExpiresAt:   time.UnixMilli(raw.ClaudeAiOauth.ExpiresAt),
	}, nil
}

func ReadCredentials(path string) (Credentials, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}
	return parseCredentials(b)
}

func ReadCredentialsKeychain(run func(name string, args ...string) ([]byte, error)) (Credentials, error) {
	b, err := run("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	if err != nil {
		return Credentials{}, err
	}
	return parseCredentials(b)
}

func loadCredentials(path string, goos string, run func(name string, args ...string) ([]byte, error)) (Credentials, error) {
	// Try file path first (works on Linux and macOS if file exists)
	c, err := ReadCredentials(path)
	if err == nil {
		return c, nil
	}
	fileErr := err
	// On macOS, fall back to Keychain
	if goos == "darwin" {
		c, keychainErr := ReadCredentialsKeychain(run)
		if keychainErr == nil {
			return c, nil
		}
		// Both failed: combine errors
		return Credentials{}, fmt.Errorf("credentials file: %v; keychain: %w", fileErr, keychainErr)
	}
	return Credentials{}, fileErr
}

func LoadCredentials() (Credentials, error) {
	return loadCredentials(DefaultCredentialsPath(), runtime.GOOS, runSecurity)
}
