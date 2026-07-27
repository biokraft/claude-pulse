# Task 10 Report: Cloudflare quick tunnel + QR pairing

## Status: Complete

## What was done
- Created `relay/internal/tunnel/tunnel.go`:
  - `ParseURL(line string) (string, bool)` — regex extraction of `https://<random>.trycloudflare.com`.
  - `PrintPairing(out io.Writer, url, token string)` — prints relay URL/token plus terminal QR of `<url>?token=<token>` via `github.com/skip2/go-qrcode`.
  - `Start(localAddr, token string, out io.Writer) (*Tunnel, error)` — execs `cloudflared tunnel --url http://<localAddr>`, scans stderr for the URL, returns an install-hint error if `cloudflared` is missing. Added timeout hardening per brief: `select` on the URL channel vs `time.After(30*time.Second)`; on timeout it stops the spawned process and returns `"timed out waiting for tunnel URL"`.
  - `(t *Tunnel) Stop()` kills the cloudflared process if running.
- Created `relay/internal/tunnel/tunnel_test.go` with the brief's two tests verbatim (`TestParseURL`, `TestPrintPairingIncludesURLAndToken`). Both cover pure logic only — no cloudflared execution in tests.
- Modified `relay/cmd/claude-pulse-relay/main.go`: replaced the `!*noTunnel` log stub with a call to `tunnel.Start(cfg.Listen, cfg.Token, os.Stdout)`; logs `"tunnel disabled: %v"` and continues serving locally on error, otherwise defers `tn.Stop()`. Added the `tunnel` import.
- Modified `relay/go.mod`/`go.sum` via `go get github.com/skip2/go-qrcode` + `go mod tidy`.

## Manual verification
`cloudflared` is not installed in this environment (`which cloudflared` → not found). Per instructions, skipped the manual QR/curl verification step and did not install anything.

## Tests
`go test ./...` → all packages pass (27 tests across 7 packages, including the 2 new tunnel tests). `go vet ./...` → clean. `gofmt -l .` → no output (all formatted).

## Commit
`38dc10c` — "feat(relay): cloudflared quick tunnel with QR pairing"

## Concerns
- None blocking. Manual end-to-end verification (real cloudflared process, QR scan, external curl) is unverified in this environment since cloudflared isn't installed — recommend running it once on a machine with cloudflared present before shipping.

## Fix: reap cloudflared process after kill

Review finding: `Stop()` killed the cloudflared process but never called `cmd.Wait()`, leaving a zombie child for the life of the relay (relay is long-running; tunnel may be started/stopped repeatedly).

Fix applied in `relay/internal/tunnel/tunnel.go`: after `t.cmd.Process.Kill()`, added `go t.cmd.Wait()` (fire-and-forget reap) with a comment explaining why. Kill's error remains ignored (best effort); reap always runs when a kill is attempted.

Verified: `gofmt -l .` clean, `go test ./internal/tunnel/...` (2 passed) and `go test ./...` (27 passed, 7 packages), `go vet ./...` clean.

Commit: `fix(relay): reap cloudflared process after kill`.
