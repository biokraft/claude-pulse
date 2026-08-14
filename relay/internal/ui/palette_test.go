package ui

import (
	"bytes"
	"testing"
)

// A service log is not a terminal, and escape codes there are noise the user
// has to read around.
func TestForNonTerminalIsPlain(t *testing.T) {
	if got := For(&bytes.Buffer{}); got != Plain() {
		t.Errorf("For(buffer) = %+v, want the empty palette", got)
	}
}
