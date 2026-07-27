// Package hook installs the Claude Code statusline hook that posts each
// statusline payload to a running claude-pulse-relay instance.
package hook

import (
	"encoding/json"
	"fmt"
	"os"
)

// Fragment returns the curl pipeline used as the statusLine command,
// posting the statusline payload to the local relay with the given token.
func Fragment(token string) string {
	return fmt.Sprintf(
		"tee >(curl -s -m 2 -X POST --data-binary @- 'http://127.0.0.1:8787/ingest/statusline?token=%s' >/dev/null) ",
		token,
	)
}

// Install adds a statusLine entry to the Claude Code settings file at
// settingsPath, creating the file (as `{}`) if it does not exist. It
// refuses, returning an error, if a statusLine entry is already present so
// it never clobbers an existing user configuration.
func Install(settingsPath string, token string) error {
	m := map[string]any{}
	if b, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if _, ok := m["statusLine"]; ok {
		return fmt.Errorf("statusLine already configured in %s; merge this manually:\n%s", settingsPath, Fragment(token))
	}

	m["statusLine"] = map[string]any{
		"type":    "command",
		"command": Fragment(token),
	}

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, b, 0o600)
}
