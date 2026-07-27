package anthropic

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type Credentials struct {
	AccessToken string
	ExpiresAt   time.Time
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

func LoadCredentials() (Credentials, error) {
	// Try file path first (works on Linux and macOS if file exists)
	c, err := ReadCredentials(DefaultCredentialsPath())
	if err == nil {
		return c, nil
	}
	// On macOS, fall back to Keychain
	if runtime.GOOS == "darwin" {
		return ReadCredentialsKeychain(func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		})
	}
	return Credentials{}, err
}
