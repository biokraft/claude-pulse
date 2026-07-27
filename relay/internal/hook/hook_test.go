package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCreatesSettingsFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := Install(path, "sekret"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	sl, ok := m["statusLine"].(map[string]any)
	if !ok {
		t.Fatalf("statusLine not set: %v", m)
	}
	if sl["type"] != "command" {
		t.Fatalf("unexpected type: %v", sl["type"])
	}
	cmd, _ := sl["command"].(string)
	if !strings.Contains(cmd, "sekret") {
		t.Fatalf("command missing token: %q", cmd)
	}
	if !strings.Contains(cmd, "127.0.0.1:8787/ingest/statusline") {
		t.Fatalf("command missing endpoint: %q", cmd)
	}
}

func TestInstallMergesIntoExistingSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"other":"value"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "tok"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	b, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["other"] != "value" {
		t.Fatalf("existing key lost: %v", m)
	}
	if _, ok := m["statusLine"]; !ok {
		t.Fatalf("statusLine not added: %v", m)
	}
}

func TestInstallRefusesWhenStatusLineExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	orig := `{"statusLine":{"type":"command","command":"echo existing"}}`
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Install(path, "tok")
	if err == nil {
		t.Fatalf("expected error when statusLine already present")
	}

	b, _ := os.ReadFile(path)
	if string(b) != orig {
		t.Fatalf("file was modified: %s", b)
	}
}

func TestFragmentContainsToken(t *testing.T) {
	f := Fragment("tok123")
	if !strings.Contains(f, "tok123") {
		t.Fatalf("fragment missing token: %s", f)
	}
}
