# Claude Pulse — PRD

## Summary

Garmin watch app (Connect IQ) showing live Claude usage: 5-hour/7-day quota
percentage, current activity status (working/idle), and daily cost/token
history. Companion to the `home-infra` dashboard's "clawd" widgets, but
targets Garmin hardware instead of a browser.

## Problem

Claude usage/quota data only lives in a browser dashboard today. No glanceable,
always-on-wrist view of "how much quota do I have left" or "is a long-running
session still working."

## Goals

- Show 5h/7d quota % and reset countdown on-watch.
- Show a simple "Claude is active" status glance.
- Show daily cost/token history in a companion view (less time-critical).
- Setup should be approachable for someone who isn't the original repo owner.

## Non-goals (v1)

- No hosted multi-tenant backend. No "sign in with your Claude account" flow —
  confirmed no such OAuth surface exists for third-party apps (see
  Constraints).
- No real-time push to the watch — polling only, on the cadence quota data
  actually changes (minutes, not seconds).
- No writing/mutating anything against the Claude account — read-only display.

## Users

Individual Claude Pro/Max or API users who run Claude Code locally and want
an on-wrist glance at remaining quota. Initially: the repo owner. Stretch:
anyone technical enough to self-host the local relay piece (see below).

## Constraints (researched, load-bearing for architecture)

- **No official third-party OAuth login exists** for a user's personal Claude
  account usage data. Confirmed via Anthropic docs + public GitHub issues.
- The only source for personal 5h/7d quota % is the **undocumented**
  `https://api.anthropic.com/api/oauth/usage` endpoint — the same one Claude
  Code's own CLI calls internally. It:
  - requires the OAuth token from the local `~/.claude/.credentials.json`
    (desktop/CLI-only, not remotely obtainable),
  - is known to 429 aggressively and unpredictably (public GitHub issues:
    anthropics/claude-code#31021, #31637),
  - pools quota per `organizationUuid`, not per individual account
    (anthropics/claude-code#41886).
- The official **Admin API** (`/v1/organizations/usage_report`,
  `/cost_report`) is real and documented, but requires an org **Admin API
  key** (`sk-ant-admin-*`), provisioned only by an org admin — not something
  an arbitrary end user can self-serve, and it reports API billing usage, not
  personal Claude Code session quota.
- **Conclusion:** every user must run a small local process themselves that
  reads their own `~/.claude/.credentials.json` and relays usage data
  somewhere the watch can reach. This is inherently self-hosted-per-user, not
  a SaaS backend.

## Architecture (proposed)

```
[user's machine]                      [Garmin watch]
  local relay daemon
    - reads ~/.claude/.credentials.json
    - polls /api/oauth/usage (≤ every 5 min, backoff on 429)
    - polls ~/.claude/jobs/*/state.json (activity signal)
    - serves small JSON HTTP endpoint  ---->  Connect IQ app
                                               polls relay via phone
                                               (Bluetooth-paired Connect
                                               Mobile app acting as bridge,
                                               OR relay exposes a public
                                               tunnel/URL if the user wants
                                               untethered polling)
```

Reuses the existing `fetch_claude_usage.py` / `fetch_claude_activity.py`
logic from `home-infra` as a starting point, repackaged as a standalone,
installable relay (not tied to home-infra's Ansible/Tailscale setup).

**Open question for design phase:** does the watch reach the relay via the
paired phone's Connect IQ companion app (standard Garmin pattern, no public
exposure needed), or via a small public tunnel (ngrok/Cloudflare Tunnel/
Tailscale) for untethered polling? Recommend starting with the
phone-companion-app pattern — it's the standard Garmin integration path and
avoids exposing anything to the internet.

## Functional requirements

1. **Watch glance/widget:** 5h % + 7d % (with a simple gauge or bar), reset
   countdown for whichever is closer to reset.
2. **Activity indicator:** boolean-derived icon/state — "working" vs "idle" —
   sourced the same way the dashboard's `claude-activity` composite signal
   works (job-dir poll OR recent quota bump OR recent token push).
3. **History view (companion/lower priority):** daily cost + token totals,
   last 7 days, in the Connect IQ companion glance-detail view.
4. **Data freshness indicator:** last-synced timestamp, since data isn't
   real-time and the source endpoint can go stale during 429 backoff windows.

## Data sources reference (from `home-infra`, for the relay to mirror)

- Quota: `GET https://api.anthropic.com/api/oauth/usage` (Bearer = OAuth
  access token from `.credentials.json`), refresh token via `claude -p "ok"`
  if near expiry.
- Activity: glob `~/.claude/jobs/*/state.json`, active if state not in
  `{done, failed}`.
- Cost/tokens: Claude Code statusline hook payload (session_id, model,
  cost.total_cost_usd, context_window token counts) — optional for v1 given
  it requires wiring the statusline hook, not just reading local state.

## Risks

- `/api/oauth/usage` is unofficial and could change or be locked down further
  without notice — no SLA, no docs, no support channel.
- Rate-limit 429s can persist for hours per public reports — watch-facing
  freshness will visibly degrade during those windows; UI must communicate
  "stale" rather than silently show wrong numbers.
- Garmin Connect IQ app review/distribution constraints not yet researched —
  TBD in implementation planning.

## Naming

App name: **Claude Pulse**.
