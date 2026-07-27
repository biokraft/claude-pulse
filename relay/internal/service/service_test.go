package service

import (
	"strings"
	"testing"
)

func TestPlistContent(t *testing.T) {
	p := PlistContent("/usr/local/bin/claude-pulse-relay")
	for _, want := range []string{"com.claudepulse.relay", "/usr/local/bin/claude-pulse-relay", "RunAtLoad"} {
		if !strings.Contains(p, want) {
			t.Fatalf("plist missing %q", want)
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
