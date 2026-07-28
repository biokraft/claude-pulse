# claude-pulse-relay

A small self-hosted daemon that feeds the **Claude Pulse** Garmin watch app with your live Claude Code usage: 5-hour and 7-day quota utilization, active-session status, and daily cost/token totals.

Your credentials never leave your machine. The relay reads your local Claude Code OAuth token, polls Anthropic's usage endpoint, and serves one tiny authenticated JSON snapshot that your watch fetches through its paired phone — over a Cloudflare quick tunnel, your own reverse proxy, or a Tailscale network. No hosted backend, no account.

```
[your machine]                                     [phone]         [watch]
claude-pulse-relay
  ├─ polls Anthropic usage API (≤ every 5 min)     Garmin          Claude Pulse
  ├─ watches ~/.claude/jobs for active sessions    Connect    ←──  background fetch
  ├─ ingests cost from a Claude Code statusline    Mobile          every 5 min
  │  hook (optional)
  └─ GET /api/v1/snapshot ── Cloudflare quick tunnel (HTTPS) ──────────┘
```

## Requirements

- macOS or Linux with [Claude Code](https://claude.com/claude-code) installed and logged in
- [`cloudflared`](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) on your PATH for the built-in tunnel (`brew install cloudflared`), unless you bring your own connectivity with `--no-tunnel`

## Install

```bash
# from source (Go 1.25+)
cd relay && go build -o claude-pulse-relay ./cmd/claude-pulse-relay

# (placeholder, once published)
brew install dinglebop/tap/claude-pulse-relay
```

## Quickstart

```bash
./claude-pulse-relay
```

On first run this generates a config at `~/.claude-pulse/config.json`
(override the directory with `CLAUDE_PULSE_HOME`) containing a random
access `token`, starts listening on `127.0.0.1:8787`, and opens a
Cloudflare quick tunnel, printing the public URL, the token, and a QR
code. Enter URL and token in **Garmin Connect → Connect IQ apps →
Claude Pulse → Settings**.

> Quick-tunnel URLs are random and rotate every time the relay restarts.
> When that happens the relay prints a fresh QR — re-enter the new URL in
> the watch settings. Run the relay as a service (below) to make restarts
> rare.

## Run as a background service

```bash
./claude-pulse-relay service install
```

This writes and loads a launchd agent (`~/Library/LaunchAgents/com.claudepulse.relay.plist`)
on macOS, or a systemd user unit (`~/.config/systemd/user/claude-pulse-relay.service`)
on Linux, pointed at the current executable, and starts it immediately.
It is safe to re-run after upgrades; it replaces any running instance.

Because a service has no terminal, the pairing QR and URL go to a log
instead:

- macOS: `tail -f ~/.claude-pulse/relay.log`
- Linux: `journalctl --user -u claude-pulse-relay -f`

```bash
./claude-pulse-relay service uninstall
```

Stops and removes the installed service.

## Cost tracking (optional)

The cost page on the watch is fed by a Claude Code statusline hook that
forwards session cost/token totals to the relay:

```bash
./claude-pulse-relay hook install
```

This adds a `statusLine` entry to `~/.claude/settings.json` that pipes the
statusline payload to the relay via `curl`. If a `statusLine` entry already
exists, `hook install` refuses to touch the file and instead prints the
command fragment so you can merge it into your existing statusline command
by hand.

Without the hook everything else works; the cost page just stays at $0.

## Running without a tunnel

If you already have a way to reach this machine — a Tailscale network,
your own reverse proxy — pass `--no-tunnel` and connect directly to
`cfg.Listen` (default `127.0.0.1:8787`; override with `-listen`):

```bash
./claude-pulse-relay --no-tunnel -listen 0.0.0.0:8787
```

The watch requires HTTPS, so front the relay with something that
terminates TLS.

## API

`GET /api/v1/snapshot?token=…` (or `Authorization: Bearer …`):

```json
{
  "five_hour_pct": 19, "seven_day_pct": 35,
  "five_hour_resets_at": "2026-07-28T11:29:59Z",
  "seven_day_resets_at": "2026-07-29T18:59:59Z",
  "is_active": true, "active_count": 3,
  "today_cost_usd": 0.42, "today_tokens": 12345,
  "daily": [{"day": "2026-07-22", "cost_usd": 0.1, "tokens": 4567}],
  "fetched_at": "2026-07-28T11:20:18Z", "stale": false
}
```

`daily` always holds 7 entries, oldest first. `stale` is true until the
first successful poll and whenever the last good poll is older than
15 minutes (e.g. during rate-limit backoff). Consumers should show stale
data as stale, not hide it.

## How it reads your credentials

- Linux: `~/.claude/.credentials.json`
- macOS: the login Keychain item `Claude Code-credentials` (falls back to
  the file if present)

The relay only ever sends the token to `api.anthropic.com`. The snapshot
endpoint never exposes it.

## Security

Every request must carry `?token=<token>` (or an `Authorization: Bearer`
header) with the token from `~/.claude-pulse/config.json`. This is your
only authentication — treat the tunnel URL and token like a password.
Unauthenticated or wrong-token requests get `401`; token comparison is
constant-time, and an empty configured token disables access entirely.
The relay binds to `127.0.0.1` by default — only the tunnel (or your own
proxy) exposes it.

To rotate the token, stop the relay, delete the `token` field from
`config.json` (or delete the whole file), and restart; a new token is
generated on next run.

## Development

```bash
cd relay
go build ./...
go test -race ./...
```

Usage data comes from an undocumented Anthropic endpoint
(`/api/oauth/usage`) and may change or disappear without notice.

## License

Apache-2.0. See [LICENSE](./LICENSE).
