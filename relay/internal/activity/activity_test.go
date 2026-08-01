package activity

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, job, state string) {
	t.Helper()
	p := filepath.Join(dir, job)
	os.MkdirAll(p, 0o755)
	os.WriteFile(filepath.Join(p, "state.json"), []byte(`{"state":"`+state+`"}`), 0o644)
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a", "running")
	write(t, dir, "b", "done")
	write(t, dir, "c", "waiting")
	active, n := Check(dir)
	if !active || n != 2 {
		t.Fatalf("got active=%v n=%d, want true 2", active, n)
	}
}

func TestCheckMissingDir(t *testing.T) {
	active, n := Check(filepath.Join(t.TempDir(), "nope"))
	if active || n != 0 {
		t.Fatal("want false, 0")
	}
}
