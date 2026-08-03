package service

import (
	"strings"
	"testing"
)

const testPath = "/opt/homebrew/bin:/usr/bin:/bin"

func TestPlistContent(t *testing.T) {
	p := PlistContent("/usr/local/bin/claude-pulse-relay", "/Users/x/.claude-pulse/relay.log", testPath)
	for _, want := range []string{"com.claudepulse.relay", "/usr/local/bin/claude-pulse-relay", "RunAtLoad"} {
		if !strings.Contains(p, want) {
			t.Fatalf("plist missing %q", want)
		}
	}
}

func TestPlistContentIncludesLogPaths(t *testing.T) {
	got := PlistContent("/usr/local/bin/claude-pulse-relay", "/Users/x/.claude-pulse/relay.log", testPath)
	for _, want := range []string{
		"<key>StandardOutPath</key><string>/Users/x/.claude-pulse/relay.log</string>",
		"<key>StandardErrorPath</key><string>/Users/x/.claude-pulse/relay.log</string>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plist missing %q:\n%s", want, got)
		}
	}
}

// Without this, launchd runs the relay with a bare PATH, cloudflared is not
// found, and the relay silently starts with no tunnel on localhost only.
func TestPlistContentSetsPath(t *testing.T) {
	got := PlistContent("/usr/local/bin/claude-pulse-relay", "/tmp/relay.log", testPath)
	if !strings.Contains(got, "<key>EnvironmentVariables</key>") {
		t.Fatalf("plist sets no environment:\n%s", got)
	}
	if !strings.Contains(got, "<key>PATH</key><string>"+testPath+"</string>") {
		t.Fatalf("plist missing PATH %q:\n%s", testPath, got)
	}
}

func TestUnitContent(t *testing.T) {
	u := UnitContent("/usr/local/bin/claude-pulse-relay", testPath)
	for _, want := range []string{
		"ExecStart=/usr/local/bin/claude-pulse-relay",
		"Restart=on-failure",
		"WantedBy=default.target",
		"Environment=PATH=" + testPath,
	} {
		if !strings.Contains(u, want) {
			t.Fatalf("unit missing %q", want)
		}
	}
}

func TestEnvPathKeepsUserOrderAndAddsBinDir(t *testing.T) {
	got := EnvPath("/usr/bin:/bin", "/opt/homebrew/bin/cloudflared")
	dirs := strings.Split(got, ":")

	if dirs[0] != "/usr/bin" || dirs[1] != "/bin" {
		t.Errorf("reordered the caller's PATH: %v", dirs)
	}
	if !strings.Contains(got, "/opt/homebrew/bin") {
		t.Errorf("did not add the required binary's directory: %q", got)
	}
}

func TestEnvPathDeduplicates(t *testing.T) {
	got := EnvPath("/usr/bin:/usr/bin:/opt/homebrew/bin", "/opt/homebrew/bin/cloudflared")
	if n := strings.Count(got, "/usr/bin"); n != 1 {
		t.Errorf("/usr/bin appears %d times in %q", n, got)
	}
	if n := strings.Count(got, "/opt/homebrew/bin"); n != 1 {
		t.Errorf("/opt/homebrew/bin appears %d times in %q", n, got)
	}
}

// An empty PATH is what a service actually gets, so the fallbacks have to
// stand on their own.
func TestEnvPathHasSaneDefaults(t *testing.T) {
	got := EnvPath("")
	for _, want := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"} {
		if !strings.Contains(got, want) {
			t.Errorf("default PATH %q missing %q", got, want)
		}
	}
}
