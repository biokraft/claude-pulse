# claude-pulse-relay

A small local relay that exposes your Claude usage (quota, activity, cost)
as a JSON snapshot you can view from your phone — over a Cloudflare quick
tunnel, your own reverse proxy, or a Tailscale network.

## Install

```bash
# from source
cd relay && go build -o claude-pulse-relay ./cmd/claude-pulse-relay

# (placeholder, once published)
brew install dinglebop/tap/claude-pulse-relay
# or
curl -fsSL https://example.com/install.sh | sh
```

## Quickstart

```bash
./claude-pulse-relay
```

On first run this generates a config at `~/.claude-pulse/config.json`
(override the directory with `CLAUDE_PULSE_HOME`) containing a random
access `token`, starts listening on `127.0.0.1:8787`, and opens a
Cloudflare quick tunnel. Scan the printed QR code (or open the printed
URL) on your phone to see live quota, activity, and cost.

## Run as a background service

```bash
./claude-pulse-relay service install
```

This writes and loads a launchd agent (`~/Library/LaunchAgents/com.claudepulse.relay.plist`)
on macOS, or a systemd user unit (`~/.config/systemd/user/claude-pulse-relay.service`)
on Linux, pointed at the current executable, and starts it immediately.

```bash
./claude-pulse-relay service uninstall
```

Stops and removes the installed service.

## Statusline hook

To feed your Claude Code statusline output (cost, model, etc.) into the
relay as well:

```bash
./claude-pulse-relay hook install
```

This adds a `statusLine` entry to `~/.claude/settings.json` that pipes the
statusline payload to the relay via `curl`. If a `statusLine` entry already
exists, `hook install` refuses to touch the file and instead prints the
command fragment so you can merge it into your existing statusline command
by hand.

## Running without a tunnel

If you already have a way to reach this machine — a Tailscale network,
your own reverse proxy — pass `--no-tunnel` and connect directly to
`cfg.Listen` (default `127.0.0.1:8787`; override with `-listen`):

```bash
./claude-pulse-relay --no-tunnel -listen 0.0.0.0:8787
```

## Security

Every request must carry `?token=<token>` (or the equivalent header) with
the token from `~/.claude-pulse/config.json`. This is your only
authentication — treat the tunnel URL and token like a password. To rotate
the token, stop the relay, delete the `token` field from `config.json` (or
delete the whole file), and restart; a new token is generated on next run.

## License

Apache-2.0. See [LICENSE](./LICENSE).
