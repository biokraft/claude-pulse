package status

import (
	"os"
	"path/filepath"
	"strings"
)

// urlFile is the name of the receipt inside the claude-pulse home directory.
const urlFile = "tunnel-url"

// RecordTunnelURL writes the relay's current public URL so a separate process
// can report it. The running relay is the only thing that knows this URL —
// cloudflared mints a new one on every start — and it used to exist solely in
// that process's memory and in whatever scrolled past in the log. `status` runs
// as its own process, so without this receipt it could not tell the user the
// one string they actually need in order to pair the watch.
//
// Errors are non-fatal: failing to write a convenience file must not stop the
// relay from serving.
func RecordTunnelURL(dir, url string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, urlFile), []byte(url+"\n"), 0o600)
}

// ClearTunnelURL removes the receipt. The relay calls this as it shuts down, so
// `status` reports "no tunnel" rather than an address that stopped resolving
// the moment cloudflared exited.
func ClearTunnelURL(dir string) {
	if dir == "" {
		return
	}
	_ = os.Remove(filepath.Join(dir, urlFile))
}

// ReadTunnelURL returns the recorded URL, or "" when no relay has recorded one.
func ReadTunnelURL(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, urlFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
