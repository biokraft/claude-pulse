// Package hook installs the Claude Code statusline hook that posts each
// statusline payload to a running claude-pulse-relay instance.
package hook

import (
	"encoding/json"
	"fmt"
	"os"
)

// Fragment returns the POSIX sh-safe statusLine command that buffers
// stdin, posts a copy to the relay at listen with the given token, and
// then emits the original payload to stdout for the statusline itself.
func Fragment(listen, token string) string {
	url := "http://" + listen + "/ingest/statusline?token=" + token
	return fmt.Sprintf(
		`sh -c 'tmp=$(cat); printf %%s "$tmp" | curl -s -m 2 -X POST --data-binary @- %s >/dev/null 2>&1; printf %%s "$tmp"'`,
		url,
	)
}

// Install adds a statusLine entry to the Claude Code settings file at
// settingsPath, creating the file (as `{}`) if it does not exist. It
// refuses, returning an error, if a statusLine entry is already present so
// it never clobbers an existing user configuration.
func Install(settingsPath string, listen string, token string) error {
	m := map[string]any{}
	if b, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if _, ok := m["statusLine"]; ok {
		return fmt.Errorf("statusLine already configured in %s; merge this manually:\n%s", settingsPath, Fragment(listen, token))
	}

	m["statusLine"] = map[string]any{
		"type":    "command",
		"command": Fragment(listen, token),
	}

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, b, 0o600)
}
