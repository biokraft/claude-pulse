# Security Policy

## Reporting a vulnerability

Please do not open a public issue for security problems.

Report privately through
[GitHub Security Advisories](https://github.com/biokraft/claude-pulse/security/advisories/new),
or by email to sean.baufeld@protonmail.com. Include what you found, how to reproduce it, and what an
attacker could do with it.

Expect an acknowledgement within a few days. This is a spare-time project, so fixes are
best-effort rather than SLA-backed — but anything touching credentials or the snapshot
endpoint gets priority.

## Threat model

The relay handles your Claude Code credentials, so the boundaries are worth stating plainly:

- **Credentials stay local.** The relay reads them from the macOS login Keychain
  (`Claude Code-credentials`) or `~/.claude/.credentials.json` on Linux, and sends them only
  to `api.anthropic.com`. They are never written to the snapshot response.
- **The snapshot endpoint is guarded by a single bearer token** stored in
  `~/.claude-pulse/config.json`. Comparison is constant-time. An empty configured token
  disables access entirely rather than allowing everyone.
- **The relay binds to `127.0.0.1` by default.** Only a tunnel or a proxy you configure
  exposes it to the network.
- **The tunnel URL plus the token is the whole authentication story.** Anyone holding both
  can read your usage snapshot: quota percentages, session activity, and daily cost. They
  cannot reach your Anthropic credentials through it. Treat the pair like a password, and
  rotate as described in the [relay README](relay/README.md).

In scope: credential leakage, authentication bypass on the snapshot endpoint, and anything
that lets a third party read or modify data through the relay.

Out of scope: the fact that a self-hosted relay is reachable when you deliberately expose
it, and the stability of Anthropic's undocumented usage endpoint.
