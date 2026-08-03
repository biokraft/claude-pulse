// Package service provides pure content generators for OS service
// definitions (launchd plist on macOS, systemd user unit on Linux) so
// claude-pulse-relay can run as a background service.
package service

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EnvPath builds the PATH a service should run with.
//
// A launchd agent inherits none of the user's shell environment: it gets a
// bare /usr/bin:/bin:/usr/sbin:/sbin. Homebrew installs cloudflared in
// /opt/homebrew/bin, so a relay that finds it fine in a terminal fails with
// "cloudflared not found" as a service, silently starting with no tunnel and
// listening only on localhost — where the watch cannot reach it. A systemd
// user unit has the same gap for /home/linuxbrew and ~/.local/bin.
//
// The fix is to capture the installing shell's PATH and guarantee the
// directories of any required binaries are on it.
func EnvPath(current string, requiredBins ...string) string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		if d == "" || d == "." || seen[d] {
			return
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	for _, d := range filepath.SplitList(current) {
		add(d)
	}
	// Append rather than prepend: the user's own ordering wins, and this only
	// adds a fallback for a binary that would otherwise be unreachable.
	for _, bin := range requiredBins {
		add(filepath.Dir(bin))
	}
	for _, d := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"} {
		add(d)
	}
	return strings.Join(dirs, string(filepath.ListSeparator))
}

// PlistContent returns a launchd plist that runs execPath at load and
// keeps it alive, with envPath as its PATH (see EnvPath).
func PlistContent(execPath, logPath, envPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.claudepulse.relay</string>
  <key>ProgramArguments</key><array><string>%s</string></array>
  <key>EnvironmentVariables</key><dict>
    <key>PATH</key><string>%s</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, execPath, envPath, logPath, logPath)
}

// UnitContent returns a systemd user unit that runs execPath, restarting
// on failure, with envPath as its PATH (see EnvPath).
func UnitContent(execPath, envPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Claude Pulse relay

[Service]
ExecStart=%s
Environment=PATH=%s
Restart=on-failure

[Install]
WantedBy=default.target
`, execPath, envPath)
}
