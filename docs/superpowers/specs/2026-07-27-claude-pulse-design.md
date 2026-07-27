# Claude Pulse — Design Spec

Date: 2026-07-27
Status: Approved (brainstorming phase)
Inputs: `PRD.md`, `design_handoff_garmin_claude_widget/README.md`

## Summary

Claude Pulse is a paid Garmin Connect IQ watch app showing live Claude Code
usage (5h/7d quota, activity, daily cost), fed by a free open-source
self-hosted relay daemon the user runs on their own machine. No hosted
backend, no accounts: the relay reads the user's local Claude credentials,
polls the undocumented usage endpoint, and exposes a small authenticated
HTTPS endpoint (via Cloudflare quick tunnel) that the watch polls through
the paired phone.

## Decisions (locked)

| Decision | Choice |
|---|---|
| Monetization | Garmin native paid app + built-in trial mode (`(:trial)`, SDK 6.2+). One-time price, Garmin Pay. $100/yr program fee, 15% cut. |
| Watch stack | Monkey C, Connect IQ SDK 8.x, VS Code extension. App type: watch-app with glance scope ("super app"). |
| Relay stack | Go, single static binary `claude-pulse-relay`. MIT-licensed open source. |
| Connectivity | Relay auto-spawns Cloudflare quick tunnel (embedded `cloudflared` spawn). Manual alternatives via `--no-tunnel --listen` documented. |
| V1 scope | All 3 pages from the design handoff, including cost/tokens page (requires statusline hook ingest). |
| Devices | System 7+ round-display devices only (paid-app requirement anyway): Fenix 7/8, Epix 2, FR 255/265/955/965, Venu 2/3, Vivoactive 5. |
| Licensing | This repo (watch app + specs) stays **private/proprietary**. `relay/` is developed here but published to a separate public repo under **Apache-2.0** at release — it reads `~/.claude/.credentials.json`, so users need source auditability. Watch source is never published; sale protection comes from the store + trial flow. |
| Naming | Keep "Claude Pulse" during development. CLAUDE is a registered US mark and Anthropic enforces sound-alikes ("Clawdbot" → forced rename, Jan 2026), so **before store submission**: request permission from marketing@anthropic.com, else rename + redesign mascot. Full analysis: `docs/ip-trademark-findings.md`. |

## Architecture

```
[user machine]                                    [phone]        [watch]
claude-pulse-relay (Go daemon)
  ├─ poller: api.anthropic.com/api/oauth/usage    Garmin       Connect IQ app
  │    (≤5 min, backoff on 429, serve-stale)      Connect  ←─  background temporal
  ├─ poller: ~/.claude/jobs/*/state.json          Mobile       event every 5 min:
  ├─ ingest: POST /ingest/statusline (hook)       (BLE/HTTPS   makeWebRequest →
  ├─ store: SQLite at ~/.claude-pulse/             bridge)     Storage → views
  └─ serve: GET /api/v1/snapshot  ──── Cloudflare quick tunnel (HTTPS) ────┘
```

Data flow is pull-only from the watch, one consolidated request per cycle
(BLE payload limits favor a single small JSON endpoint).

## Component: watch app (`watch/`)

- **Glance** (32 kB hard memory budget): renders "CLAUDE 68% · 42%" strip
  plus stale indicator. Reads pre-computed values from
  `Application.Storage` only; zero fetch/format logic in glance scope.
- **Background service** (`Toybox.Background`): temporal event every 5 min
  (platform minimum) → `Communications.makeWebRequest` to
  `GET {relayUrl}/api/v1/snapshot?token={token}` → `Background.exit(data)`;
  foreground `onBackgroundData` writes to Storage.
- **Views** (3 swipeable pages per design handoff, high fidelity):
  1. Glance page: two donut rings (5H/7D) via `dc.drawArc`, accent
     `#CC7A56`, warning `#C24B3A` at ≥85%.
  2. Quota detail: mascot sprite, "{n} job(s) running", per-window progress
     rows with reset countdowns.
  3. Today's cost: dollar figure, token caption, 7-day mini bar chart.
  Hardware buttons (UP/DOWN page, BACK to page 1) and touch both supported.
- **Mascot pose engine**: full priority logic from handoff
  (celebrate → annoyed → working → sleeping → idle). Only
  `clawd-idle-look` asset exists today: v1 renders that sprite for all
  poses, with pose communicated via label text/color, until the sprite
  pack is delivered. Sprites ship as PNG bitmaps (Connect IQ has no
  GIF/SVG support); animation later via multi-frame bitmaps + `Timer`,
  battery-capped.
- **Staleness**: every snapshot carries `fetched_at`; views show
  "synced Xm ago" and dim rings when data >15 min old. 429 windows
  degrade visibly, never silently wrong.
- **Trial**: `(:trial)` annotation; trial = fully functional, store
  handles purchase/unlock.
- **Settings** (Garmin Connect app-settings): relay URL, relay token,
  accent color choice.

## Component: relay (`relay/`)

- `internal/anthropic`: polls `GET /api/oauth/usage` with Bearer token from
  `~/.claude/.credentials.json`; refresh handling near expiry; exponential
  backoff on 429; always serves last-good snapshot with `stale: true`.
- `internal/activity`: globs `~/.claude/jobs/*/state.json`; active if state
  not in `{done, failed}`.
- `internal/ingest`: `POST /ingest/statusline` accepts Claude Code
  statusline hook payload (cost, tokens, model); installer offers to wire
  the hook into `~/.claude/settings.json`.
- Store: SQLite single file under `~/.claude-pulse/` (daily rollups for the
  7-day chart; snapshots need no history).
- API: `GET /api/v1/snapshot` →
  `{five_hour_pct, seven_day_pct, five_hour_resets_at, seven_day_resets_at,
  is_active, active_count, today_cost_usd, today_tokens, daily[7],
  fetched_at, stale}`.
- **Auth**: random bearer token generated on first run, stored in relay
  config; required as query param on every request. Tunnel URL is public,
  so the token is mandatory, not optional.
- **Tunnel**: spawns `cloudflared` quick tunnel on start; prints URL +
  token as QR code and plain text for entry into Garmin Connect settings.
  Quick-tunnel URLs rotate on restart: relay detects the new URL and
  re-prints the QR. `--no-tunnel --listen :port` for Tailscale/own-proxy
  users.
- Install: Homebrew formula + curl install script;
  `claude-pulse-relay service install` sets up launchd (macOS) / systemd
  (Linux).
- License: Apache-2.0, mirrored to a public repo at release (private repo
  is the source of truth during development).

## Testing

- **Watch unit tests**: Run No Evil (`(:test)`) over pure logic extracted
  into `model/` — pose priority, reset-countdown formatting, staleness
  math, chart scaling. CI: `matco/connectiq-tester` Docker image + xvfb in
  GitHub Actions.
- **Watch manual**: simulator device profiles (Fenix 8, FR 965, Venu 3)
  including glance mode and manually-fired temporal events; then sideload
  debug `.prg` to real hardware before every store submission (simulator
  pass ≠ device pass, especially background memory). Store beta channel
  before public listing.
- **Relay**: `go test` with a mocked Anthropic endpoint covering 429
  sequences, token refresh, stale serving, and ingest rollups.

## Repo structure

```
claude-pulse/
├── PRD.md
├── README.md
├── .gitignore
├── docs/superpowers/specs/
├── design_handoff_garmin_claude_widget/   # reference only, unchanged
├── watch/
│   ├── manifest.xml
│   ├── monkey.jungle
│   ├── source/
│   │   ├── ClaudePulseApp.mc
│   │   ├── glance/
│   │   ├── views/
│   │   ├── model/
│   │   └── background/
│   ├── resources/
│   └── tests/
├── relay/
│   ├── cmd/claude-pulse-relay/
│   ├── internal/
│   │   ├── anthropic/
│   │   ├── activity/
│   │   ├── ingest/
│   │   ├── server/
│   │   └── tunnel/
│   └── go.mod
└── .github/workflows/ci.yml
```

`.gitignore`: `bin/`, `dist/`, `*.prg`, `*.prg.debug.xml`, `gen/`,
`.DS_Store`, `developer_key*`, Go build output. The Connect IQ developer
signing key is never committed.

## Risks accepted

- `/api/oauth/usage` is undocumented and may change or be locked down
  without notice (carried from PRD).
- Cloudflare quick-tunnel URL rotation forces occasional settings
  re-entry; mitigated by QR re-print, not eliminated.
- Mascot sprite pack (19 files) undelivered; v1 ships with the single
  idle sprite standing in for all poses.
- Paid-app store review is stricter and can temporarily delist during
  re-review.
