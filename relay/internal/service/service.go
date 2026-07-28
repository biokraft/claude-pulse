// Package service provides pure content generators for OS service
// definitions (launchd plist on macOS, systemd user unit on Linux) so
// claude-pulse-relay can run as a background service.
package service

import "fmt"

// PlistContent returns a launchd plist that runs execPath at load and
// keeps it alive.
func PlistContent(execPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.claudepulse.relay</string>
  <key>ProgramArguments</key><array><string>%s</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, execPath, logPath, logPath)
}

// UnitContent returns a systemd user unit that runs execPath, restarting
// on failure.
func UnitContent(execPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Claude Pulse relay

[Service]
ExecStart=%s
Restart=on-failure

[Install]
WantedBy=default.target
`, execPath)
}
