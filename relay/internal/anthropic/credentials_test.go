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

func TestLoadCredentialsFileExists(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".credentials.json")
	os.WriteFile(p, []byte(`{"claudeAiOauth":{"accessToken":"file-tok","expiresAt":1785000000000}}`), 0o600)

	runnerCalled := false
	fakeRunner := func(name string, args ...string) ([]byte, error) {
		runnerCalled = true
		t.Fatal("runner should not be called when file exists")
		return nil, errors.New("should not reach")
	}

	c, err := loadCredentials(p, "darwin", fakeRunner)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "file-tok" {
		t.Fatalf("token = %q", c.AccessToken)
	}
	if runnerCalled {
		t.Fatal("runner should not be called when file exists")
	}
}

func TestLoadCredentialsKeychainFallback(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nonexistent.json") // File doesn't exist

	fakeRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(`{"claudeAiOauth":{"accessToken":"keychain-tok","expiresAt":1785000000000}}`), nil
	}

	c, err := loadCredentials(p, "darwin", fakeRunner)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "keychain-tok" {
		t.Fatalf("token = %q", c.AccessToken)
	}
}

func TestLoadCredentialsLinuxNoFallback(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nonexistent.json") // File doesn't exist

	runnerCalled := false
	fakeRunner := func(name string, args ...string) ([]byte, error) {
		runnerCalled = true
		t.Fatal("runner should not be called on linux")
		return nil, errors.New("should not reach")
	}

	_, err := loadCredentials(p, "linux", fakeRunner)
	if err == nil {
		t.Fatal("want error")
	}
	if runnerCalled {
		t.Fatal("runner should not be called on linux")
	}
}

func TestLoadCredentialsBothFail(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nonexistent.json") // File doesn't exist

	fakeRunner := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("keychain failed")
	}

	_, err := loadCredentials(p, "darwin", fakeRunner)
	if err == nil {
		t.Fatal("want error")
	}
	// Error should mention both sources
	errStr := err.Error()
	if !contains(errStr, "credentials file") || !contains(errStr, "keychain") {
		t.Fatalf("error should mention both file and keychain: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
