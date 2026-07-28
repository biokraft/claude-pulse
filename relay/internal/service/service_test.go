package service

import (
	"strings"
	"testing"
)

func TestPlistContent(t *testing.T) {
	p := PlistContent("/usr/local/bin/claude-pulse-relay", "/Users/x/.claude-pulse/relay.log")
	for _, want := range []string{"com.claudepulse.relay", "/usr/local/bin/claude-pulse-relay", "RunAtLoad"} {
		if !strings.Contains(p, want) {
			t.Fatalf("plist missing %q", want)
		}
	}
}

func TestPlistContentIncludesLogPaths(t *testing.T) {
	got := PlistContent("/usr/local/bin/claude-pulse-relay", "/Users/x/.claude-pulse/relay.log")
	for _, want := range []string{
		"<key>StandardOutPath</key><string>/Users/x/.claude-pulse/relay.log</string>",
		"<key>StandardErrorPath</key><string>/Users/x/.claude-pulse/relay.log</string>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plist missing %q:\n%s", want, got)
		}
	}
}

func TestUnitContent(t *testing.T) {
	u := UnitContent("/usr/local/bin/claude-pulse-relay")
	for _, want := range []string{"ExecStart=/usr/local/bin/claude-pulse-relay", "Restart=on-failure", "WantedBy=default.target"} {
		if !strings.Contains(u, want) {
			t.Fatalf("unit missing %q", want)
		}
	}
}
