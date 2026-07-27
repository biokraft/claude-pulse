package anthropic

import (
	"errors"
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

func TestParseCredentials(t *testing.T) {
	b := []byte(`{"claudeAiOauth":{"accessToken":"tok-abc","expiresAt":1785000000000}}`)
	c, err := parseCredentials(b)
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

func TestParseCredentialsInvalid(t *testing.T) {
	if _, err := parseCredentials([]byte("not json")); err == nil {
		t.Fatal("want error")
	}
}

func TestReadCredentialsKeychain(t *testing.T) {
	fakeRunner := func(name string, args ...string) ([]byte, error) {
		if name != "security" || len(args) < 4 || args[0] != "find-generic-password" {
			t.Fatalf("unexpected runner call: %s %v", name, args)
		}
		return []byte(`{"claudeAiOauth":{"accessToken":"keychain-tok","expiresAt":1785000000000}}`), nil
	}
	c, err := ReadCredentialsKeychain(fakeRunner)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "keychain-tok" {
		t.Fatalf("token = %q", c.AccessToken)
	}
}

func TestReadCredentialsKeychainError(t *testing.T) {
	fakeRunner := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("keychain error")
	}
	if _, err := ReadCredentialsKeychain(fakeRunner); err == nil {
		t.Fatal("want error")
	}
}
