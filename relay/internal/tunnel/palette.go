package tunnel

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// The Anthropic palette, the same hex values the watch app draws with (see
// watch/source/views/Chrome.mc). 24-bit escapes are used because the clay
// accent has no reasonable 16-colour equivalent; xterm-256 approximations are
// the fallback.
type palette struct {
	Clay, Cream, Muted, Bold, Dim, Reset string
}

var (
	truecolor = palette{
		Clay:  "\033[38;2;204;122;86m",  // #CC7A56
		Cream: "\033[38;2;247;245;242m", // #F7F5F2
		Muted: "\033[38;2;169;163;153m", // #A9A399
		Bold:  "\033[1m",
		Dim:   "\033[2m",
		Reset: "\033[0m",
	}
	ansi256 = palette{
		Clay:  "\033[38;5;173m",
		Cream: "\033[38;5;255m",
		Muted: "\033[38;5;247m",
		Bold:  "\033[1m",
		Dim:   "\033[2m",
		Reset: "\033[0m",
	}
	plain = palette{}
)

// paletteFor picks the richest palette the destination can actually render.
// Colour is suppressed unless w is a terminal, so the pairing block stays
// readable in a service log — which is exactly where it lands once the relay
// runs under launchd or systemd.
func paletteFor(w io.Writer) palette {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return plain
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return plain
	}
	switch strings.ToLower(os.Getenv("COLORTERM")) {
	case "truecolor", "24bit":
		return truecolor
	}
	return ansi256
}
