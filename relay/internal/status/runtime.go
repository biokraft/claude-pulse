package status

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// runtimeFile is the name of the receipt inside the claude-pulse home directory.
const runtimeFile = "runtime.json"

// Runtime is what a running relay records about itself so a second process can
// find and describe it.
//
// Every field is here because config.json cannot answer the question. The
// public URL is minted fresh by cloudflared on every start and lives only in
// the running process. The listen address and the tunnel decision can both be
// overridden by flags that are never written back to the config — so a `status`
// that trusted the config would probe the wrong port, and would report a relay
// deliberately started with --no-tunnel as broken.
type Runtime struct {
	Listen string `json:"listen"`
	URL    string `json:"url,omitempty"`
	Tunnel bool   `json:"tunnel"`
}

// RecordRuntime writes the receipt. Errors are non-fatal: failing to write a
// file that exists only to help diagnostics must not stop the relay serving.
func RecordRuntime(dir string, rt Runtime) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	b, err := json.Marshal(rt)
	if err != nil {
		return
	}
	// Write via a temp file so a `status` reading concurrently sees either the
	// old receipt or the new one, never a half-written one.
	tmp := filepath.Join(dir, runtimeFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, filepath.Join(dir, runtimeFile)); err != nil {
		_ = os.Remove(tmp)
	}
}

// ClearRuntime removes the receipt as the relay shuts down, so `status` reports
// "not running" rather than an address that stopped answering.
//
// A killed or crashed relay leaves the receipt behind. That is handled by
// probing the recorded address rather than trusting the file: a stale receipt
// fails its probe and is reported as a relay that is not running.
func ClearRuntime(dir string) {
	if dir == "" {
		return
	}
	_ = os.Remove(filepath.Join(dir, runtimeFile))
}

// ReadRuntime returns the recorded receipt, and whether there was a usable one.
func ReadRuntime(dir string) (Runtime, bool) {
	b, err := os.ReadFile(filepath.Join(dir, runtimeFile))
	if err != nil {
		return Runtime{}, false
	}
	var rt Runtime
	if err := json.Unmarshal(b, &rt); err != nil {
		return Runtime{}, false
	}
	return rt, rt.Listen != ""
}
