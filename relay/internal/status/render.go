package status

import (
	"fmt"
	"io"
	"time"

	"github.com/biokraft/claude-pulse/relay/internal/ui"
)

// Render writes the report as a terminal page in the same Anthropic palette as
// the pairing block and install.sh.
//
// The token is never printed. `status` is the command a user runs when asking
// for help, so its output is the most likely thing in the project to be pasted
// into a public issue — the URL alone is harmless without the token, and the
// issue templates already ask for neither.
func Render(w io.Writer, r Report) {
	p := ui.For(w)

	section := func(title string) {
		fmt.Fprintf(w, "\n%s%s  %s%s\n", p.Clay, p.Bold, title, p.Reset)
	}
	line := func(label, value string) {
		fmt.Fprintf(w, "  %s%-12s%s %s%s%s\n", p.Muted, label, p.Reset, p.Cream, value, p.Reset)
	}
	good := func(label, value string) {
		fmt.Fprintf(w, "  %s%-12s%s %s✓%s %s%s%s\n",
			p.Muted, label, p.Reset, p.Sage, p.Reset, p.Cream, value, p.Reset)
	}
	bad := func(label, value string) {
		fmt.Fprintf(w, "  %s%-12s%s %s✗%s %s%s%s\n",
			p.Muted, label, p.Reset, p.Rust, p.Reset, p.Cream, value, p.Reset)
	}

	fmt.Fprintf(w, "\n%s%s  Claude Pulse — relay status%s\n", p.Clay, p.Bold, p.Reset)

	section("Relay")
	if r.LocalUp {
		good("listening", r.Listen)
		if r.ListenOverride {
			line("", "started with -listen, so this is not the address in the config")
		}
	} else {
		bad("listening", "nothing answers on "+r.Listen)
		if r.LocalErr != "" {
			line("", r.LocalErr)
		}
	}
	if r.TokenLen > 0 {
		good("token", fmt.Sprintf("set (%d characters, not shown)", r.TokenLen))
	} else {
		bad("token", "missing from "+r.ConfigPath)
	}
	line("config", r.ConfigPath)

	section("Watch reachability")
	switch {
	case r.NoTunnel && r.TunnelURL == "":
		line("tunnel", "none of its own (--no-tunnel) — something else must front the relay")
		line("", "check that route with: claude-pulse-relay status -url https://your-host")
	case r.TunnelURL == "":
		bad("tunnel", "not running")
	case r.TunnelUp:
		good("tunnel", r.TunnelURL)
		if !r.TunnelGiven {
			line("", "this is the URL to enter in the watch's settings")
		}
	default:
		bad("tunnel", r.TunnelURL)
		line("", r.TunnelErr)
	}

	// The decisive question when the watch shows dashes: has it ever actually
	// reached the relay? A reachable tunnel does not prove it has.
	if r.Snap != nil {
		switch {
		case r.Snap.ServedLastAt != "":
			good("watch fetch", ageOf(r.Snap.ServedLastAt, r.Now)+" ("+orDash(r.Snap.ServedAgent)+")")
		case r.Snap.DeniedLastAt != "":
			bad("watch fetch", "never — but something was rejected "+
				ageOf(r.Snap.DeniedLastAt, r.Now)+", so the token is wrong")
		default:
			bad("watch fetch", "never since the relay started")
		}
	}

	section("Usage data")
	if r.Snap == nil {
		bad("snapshot", orDash(r.SnapErr))
	} else {
		if r.Snap.Stale {
			bad("fetched", ageOf(r.Snap.FetchedAt, r.Now))
		} else {
			good("fetched", ageOf(r.Snap.FetchedAt, r.Now))
		}
		if r.Snap.QuotaSource != "" {
			// Which source is feeding the watch decides what a problem here
			// even means: the statusline cannot be rate limited, so a backoff
			// only matters while the poll is the one being read.
			line("source", sourceLabel(r.Snap.QuotaSource))
		}
		line("5-hour", fmt.Sprintf("%.0f%%", r.Snap.FiveHourPct))
		line("7-day", fmt.Sprintf("%.0f%%", r.Snap.SevenDayPct))
		line("today", fmt.Sprintf("$%.2f, %d tokens", r.Snap.TodayCostUSD, r.Snap.TodayTokens))
		line("sessions", fmt.Sprintf("%d active", r.Snap.ActiveCount))
	}
	if !r.NextPoll.IsZero() {
		d := r.NextPoll.Sub(r.Now)
		switch {
		case d <= 0:
			line("next poll", "due now")
		default:
			line("next poll", "in "+short(d))
		}
		// An interval above the 5-minute base is the only visible trace of an
		// Anthropic rate limit, which otherwise looks exactly like a relay
		// that has simply stopped working.
		if r.PollInterval > 5*time.Minute {
			line("", fmt.Sprintf("backing off — polling every %s instead of 5m", short(r.PollInterval)))
		}
	}

	section("Integration")
	if r.ServiceInstalled {
		good("service", r.ServicePath)
	} else {
		line("service", "not installed — the relay only runs while a terminal is open")
	}
	// Whether the hook is wired up is inferred from whether cost has actually
	// arrived. Looking for our command in Claude Code's settings answers a
	// different question: it misses a user who chains the post inside their own
	// statusline script, and passes a settings entry that posts nowhere.
	switch {
	case r.Snap == nil:
		line("cost data", "unknown — the snapshot could not be read")
	case r.Snap.CostLastAt == "":
		line("cost data", "never received — cost and token pages stay empty ('hook install')")
	default:
		good("cost data", ageOf(r.Snap.CostLastAt, r.Now)+" (statusline hook is working)")
	}

	problems := r.Problems()
	fmt.Fprintln(w)
	if len(problems) == 0 {
		fmt.Fprintf(w, "  %s%s✓ everything looks healthy%s\n\n", p.Sage, p.Bold, p.Reset)
		return
	}
	fmt.Fprintf(w, "  %s%sProblems%s\n", p.Rust, p.Bold, p.Reset)
	for _, s := range problems {
		fmt.Fprintf(w, "  %s•%s %s%s%s\n", p.Rust, p.Reset, p.Cream, s, p.Reset)
	}
	fmt.Fprintln(w)
}

func sourceLabel(src string) string {
	if src == "statusline" {
		return "Claude Code statusline (no API poll needed)"
	}
	return "Anthropic usage API"
}

func orDash(s string) string {
	if s == "" {
		return "unavailable"
	}
	return s
}

// ageOf turns the snapshot's RFC3339 timestamp into something a human reads at
// a glance. The zero time is what the relay reports before its first successful
// poll, and printing it raw ("0001-01-01T00:00:00Z") was actively confusing.
func ageOf(ts string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil || t.IsZero() || t.Year() < 2000 {
		return "never — no successful poll yet"
	}
	return short(now.Sub(t)) + " ago"
}

// short renders a duration the way the watch does: the two largest units, no
// more. time.Duration's own String gives "8h12m30.0021s".
func short(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}
