package anthropic

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCredentials(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".credentials.json")
	os.WriteFile(p, []byte(`{"claudeAiOauth":{"accessToken":"tok-abc","expiresAt":1785000000000}}`), 0o600)
	c, err := ReadCredentials(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "tok-abc" {
		t.Fatalf("token = %q", c.AccessToken)
	}
	want := time.UnixMilli(1785000000000)
	if !c.ExpiresAt.Equal(want) {
		t.Fatalf("expires = %v, want %v", c.ExpiresAt, want)
	}
}

func TestReadCredentialsMissingFile(t *testing.T) {
	if _, err := ReadCredentials(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("want error")
	}
}
