package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func Check(jobsDir string) (bool, int) {
	matches, _ := filepath.Glob(filepath.Join(jobsDir, "*", "state.json"))
	n := 0
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		var s struct {
			State string `json:"state"`
		}
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		if s.State != "done" && s.State != "failed" {
			n++
		}
	}
	return n > 0, n
}
