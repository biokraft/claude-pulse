// Package ui holds the terminal presentation shared by everything the relay
// prints: the pairing block, and the status report. It lives apart from those
// callers so the Anthropic palette is defined exactly once.
package ui

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Palette is the Anthropic palette, the same hex values the watch app draws
// with (see watch/source/views/Chrome.mc). 24-bit escapes are used because the
// clay accent has no reasonable 16-colour equivalent; xterm-256 approximations
// are the fallback.
type Palette struct {
	Clay, Cream, Muted, Sage, Rust, Bold, Dim, Reset string
}

var (
	truecolor = Palette{
		Clay:  "\033[38;2;204;122;86m",  // #CC7A56
		Cream: "\033[38;2;247;245;242m", // #F7F5F2
		Muted: "\033[38;2;169;163;153m", // #A9A399
		Sage:  "\033[38;2;111;154;106m", // #6F9A6A
		Rust:  "\033[38;2;193;90;62m",   // #C15A3E
		Bold:  "\033[1m",
		Dim:   "\033[2m",
		Reset: "\033[0m",
	}
	ansi256 = Palette{
		Clay:  "\033[38;5;173m",
		Cream: "\033[38;5;255m",
		Muted: "\033[38;5;247m",
		Sage:  "\033[38;5;108m",
		Rust:  "\033[38;5;167m",
		Bold:  "\033[1m",
		Dim:   "\033[2m",
		Reset: "\033[0m",
	}
	plain = Palette{}
)

// Plain returns the palette that emits no escape sequences.
func Plain() Palette { return plain }

// For picks the richest palette the destination can actually render. Colour is
// suppressed unless w is a terminal, so output stays readable in a service log
// — which is exactly where it lands once the relay runs under launchd or
// systemd.
func For(w io.Writer) Palette {
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
