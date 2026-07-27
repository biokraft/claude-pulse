package anthropic

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func ReadCredentials(path string) (Credentials, error) {
	var raw struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Credentials{}, err
	}
	return Credentials{
		AccessToken: raw.ClaudeAiOauth.AccessToken,
		ExpiresAt:   time.UnixMilli(raw.ClaudeAiOauth.ExpiresAt),
	}, nil
}
