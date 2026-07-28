# Relay Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the five residual findings recorded after the relay's final whole-branch review: poller backoff reset, ingest seen-map pruning, service-mode pairing visibility, keychain error detail, and service-install idempotency.

**Architecture:** All changes are small, local refinements to existing packages (`internal/anthropic`, `internal/server`, `internal/service`, `cmd/claude-pulse-relay`). No new packages, no API changes to the frozen `/api/v1/snapshot` contract.

**Tech Stack:** Go 1.25, stdlib only (existing deps unchanged).

## Global Constraints

- Go module: `github.com/dinglebop/claude-pulse/relay`, Go 1.25.
- The `/api/v1/snapshot` JSON contract is frozen — no field changes.
- All tests must pass with `go test -race ./...` from `relay/`.
- TDD: write the failing test first for every behavior change.
- Commit messages: conventional commits (`fix:`, `feat:`, `test:`).

---

### Task 1: Poller — reset backoff interval on non-429 errors

**Files:**
- Modify: `relay/internal/anthropic/usage.go`
- Test: `relay/internal/anthropic/usage_test.go`

**Interfaces:**
- Consumes: existing `UsagePoller` (`Poll(now)`, `Current(now)`).
- Produces: no signature changes. Behavior change only: any non-429 poll outcome (credentials error, transport error, non-200 status, decode error, or success) resets `p.interval` to `baseInterval`. Only consecutive 429s keep the interval escalated.

Rationale: today a 429 doubles `interval`, but a later transport error or non-200 leaves the escalated `interval` in place while scheduling `nextDue` at `baseInterval` — a following 429 then doubles from the stale escalated value (e.g. 20 min → 40 min after only one new 429). Escalation state should not survive a non-429 outcome.

- [ ] **Step 1: Write the failing test**

Append to `usage_test.go` (match existing test-server helpers in that file; adapt helper names to what is already there):

```go
func TestPollNon429ErrorResetsBackoff(t *testing.T) {
	// Server returns: 429, 500, 429. After the 500, escalation must
	// restart from baseInterval, so the final 429 yields 10 min, not 20.
	codes := []int{429, 500, 429}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(codes[i])
		i++
	}))
	defer srv.Close()

	p := NewUsagePoller(srv.URL, func() (Credentials, error) {
		return Credentials{AccessToken: "tok"}, nil
	})
	t0 := time.Now()
	p.Poll(t0) // 429 → interval 10m, nextDue t0+10m
	p.Poll(t0.Add(11 * time.Minute)) // 500 → interval resets to 5m
	t2 := t0.Add(17 * time.Minute)
	p.Poll(t2) // 429 → interval 10m (5m*2), NOT 20m

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.interval != 10*time.Minute {
		t.Fatalf("interval = %v, want 10m (escalation must restart after non-429 error)", p.interval)
	}
	if !p.nextDue.Equal(t2.Add(10 * time.Minute)) {
		t.Fatalf("nextDue = %v, want %v", p.nextDue, t2.Add(10*time.Minute))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd relay && go test ./internal/anthropic/ -run TestPollNon429ErrorResetsBackoff -v`
Expected: FAIL — interval is 20m because the 500 did not reset it.

- [ ] **Step 3: Implement**

In `usage.go`, add `p.interval = baseInterval` inside each non-429 error branch (creds error, `client.Do` error, non-200 non-429 status, decode error). The success branch already resets it. Example for one branch:

```go
	if resp.StatusCode != http.StatusOK {
		p.mu.Lock()
		p.interval = baseInterval
		p.nextDue = now.Add(baseInterval)
		p.mu.Unlock()
		return
	}
```

Apply the same two lines (`p.interval = baseInterval` before `p.nextDue = ...`) in the creds-error, transport-error, and decode-error branches.

- [ ] **Step 4: Run full package tests with race detector**

Run: `cd relay && go test -race ./internal/anthropic/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add relay/internal/anthropic/usage.go relay/internal/anthropic/usage_test.go
git commit -m "fix(relay): reset poller backoff interval on non-429 outcomes"
```

---

### Task 2: Ingest — prune idle sessions from seen map

**Files:**
- Modify: `relay/internal/server/ingest.go`
- Test: `relay/internal/server/ingest_test.go`

**Interfaces:**
- Consumes: `store.Store.AddCost(day string, cost float64, tokens int64) error`.
- Produces: `IngestHandler(st *store.Store, today func() string, now func() time.Time) http.Handler` — **signature gains a `now func() time.Time` parameter**. Update the call site in `relay/internal/server/server.go` (or wherever `IngestHandler` is wired; find with `rg -n "IngestHandler" relay/`) to pass `time.Now`.

Behavior: each `sessionSeen` entry records `last time.Time`. On every successful ingest, after advancing the seen map, delete entries whose `last` is older than 24h. Bounds memory for a long-running daemon; 24h idle sessions can safely restart delta tracking from zero (worst case: one full-total recount treated as fresh, same as the existing restart path).

- [ ] **Step 1: Write the failing test**

Append to `ingest_test.go` (reuse the existing test store/setup helpers already in that file):

```go
func TestIngestPrunesIdleSessions(t *testing.T) {
	st := newTestStore(t) // reuse existing helper; adapt name if different
	cur := time.Unix(1_000_000, 0)
	h := IngestHandler(st, func() string { return "2026-07-28" }, func() time.Time { return cur })

	post := func(sessionID string, cost float64) {
		body := fmt.Sprintf(`{"session_id":%q,"cost":{"total_cost_usd":%f},"context_window":{"input_tokens":10,"output_tokens":5}}`, sessionID, cost)
		req := httptest.NewRequest("POST", "/ingest/statusline", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d", w.Code)
		}
	}

	post("old-session", 1.00)
	cur = cur.Add(25 * time.Hour)
	post("new-session", 0.50) // triggers prune of old-session

	// old-session was pruned: same cumulative total counts fresh again
	// (delta = full amount, not zero).
	post("old-session", 1.00)

	got, err := st.Daily("2026-07-28", 1)
	if err != nil {
		t.Fatal(err)
	}
	// 1.00 + 0.50 + 1.00 (re-counted after prune) = 2.50
	if got[0].CostUSD != 2.50 {
		t.Fatalf("cost = %v, want 2.50 (old-session must have been pruned)", got[0].CostUSD)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd relay && go test ./internal/server/ -run TestIngestPrunesIdleSessions -v`
Expected: FAIL — compile error (`IngestHandler` has no `now` param) or cost 1.50 once wired.

- [ ] **Step 3: Implement**

In `ingest.go`:

```go
const seenTTL = 24 * time.Hour

type sessionSeen struct {
	cost   float64
	tokens int64
	last   time.Time
}

func IngestHandler(st *store.Store, today func() string, now func() time.Time) http.Handler {
	var mu sync.Mutex
	seen := map[string]sessionSeen{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ... existing decode / delta logic unchanged ...
		t := now()
		// after successful st.AddCost:
		seen[p.SessionID] = sessionSeen{cost: p.Cost.TotalCostUSD, tokens: total, last: t}
		for id, s := range seen {
			if t.Sub(s.last) > seenTTL {
				delete(seen, id)
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}
```

Update the `IngestHandler` call site to pass `time.Now`. Fix any other existing tests that call `IngestHandler` (add `func() time.Time { return time.Now() }` or `time.Now`).

- [ ] **Step 4: Run full package tests with race detector**

Run: `cd relay && go test -race ./internal/server/ -v`
Expected: all PASS (including pre-existing ingest tests updated for the new param).

- [ ] **Step 5: Commit**

```bash
git add relay/internal/server/
git commit -m "fix(relay): prune idle sessions from ingest seen map after 24h"
```

---

### Task 3: Service mode — log paths + pairing visibility

**Files:**
- Modify: `relay/internal/service/service.go`
- Modify: `relay/cmd/claude-pulse-relay/main.go` (service install messaging + plist call site)
- Test: `relay/internal/service/service_test.go`

**Interfaces:**
- Consumes: `config` defaults (`$CLAUDE_PULSE_HOME`, default `~/.claude-pulse`).
- Produces: `PlistContent(execPath, logPath string) string` — **signature gains `logPath`**. `UnitContent(execPath string) string` unchanged (systemd already journals stdout).

Behavior: launchd services have no terminal, so the pairing QR printed at startup vanishes. Fix: plist sets `StandardOutPath`/`StandardErrorPath` to `logPath` (`~/.claude-pulse/relay.log`), and `service install` prints where to find pairing info.

- [ ] **Step 1: Write the failing test**

Append to `service_test.go`:

```go
func TestPlistContentIncludesLogPaths(t *testing.T) {
	got := PlistContent("/usr/local/bin/claude-pulse-relay", "/Users/x/.claude-pulse/relay.log")
	for _, want := range []string{
		"<key>StandardOutPath</key><string>/Users/x/.claude-pulse/relay.log</string>",
		"<key>StandardErrorPath</key><string>/Users/x/.claude-pulse/relay.log</string>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plist missing %q:\n%s", want, got)
		}
	}
}
```

Update any existing `PlistContent` tests to the two-arg signature.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd relay && go test ./internal/service/ -v`
Expected: FAIL to compile (one-arg signature).

- [ ] **Step 3: Implement**

```go
func PlistContent(execPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.claudepulse.relay</string>
  <key>ProgramArguments</key><array><string>%s</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, execPath, logPath, logPath)
}
```

In `main.go` `runServiceCmd` install branch (darwin): compute `logPath := filepath.Join(pulseHome, "relay.log")` where `pulseHome` is `$CLAUDE_PULSE_HOME` if set, else `filepath.Join(home, ".claude-pulse")` (mirror `config` package's home resolution — check `relay/internal/config/config.go` and reuse an exported helper if one exists; if not, export one, e.g. `config.Home() (string, error)`, and use it in both places). Pass to `service.PlistContent(exePath, logPath)`. After successful install print:

```go
fmt.Printf("installed and started service: %s\n", path)
if runtime.GOOS == "darwin" {
	fmt.Printf("pairing QR + URL will appear in: %s\n   view with: tail -f %s\n", logPath, logPath)
} else {
	fmt.Println("pairing QR + URL: journalctl --user -u claude-pulse-relay -f")
}
```

- [ ] **Step 4: Run tests + build**

Run: `cd relay && go test -race ./internal/service/ -v && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add relay/internal/service/ relay/cmd/claude-pulse-relay/main.go relay/internal/config/
git commit -m "feat(relay): write service logs to relay.log and point users at pairing info"
```

---

### Task 4: Keychain — include stderr in error messages

**Files:**
- Modify: `relay/internal/anthropic/credentials.go`
- Test: `relay/internal/anthropic/credentials_test.go`

**Interfaces:**
- Consumes/Produces: no exported signature changes. `LoadCredentials()`'s internal `run` closure switches from `.Output()` error passthrough to wrapping `*exec.ExitError` stderr into the error text.

Behavior: `security find-generic-password` failures currently surface as `exit status 44` with no explanation. `cmd.Output()` populates `ExitError.Stderr`; include it.

- [ ] **Step 1: Write the failing test**

Append to `credentials_test.go`:

```go
func TestRunSecurityWrapsStderr(t *testing.T) {
	_, err := runSecurity("sh", "-c", `echo "The specified item could not be found in the keychain." >&2; exit 44`)
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "could not be found in the keychain") {
		t.Fatalf("error missing stderr detail: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd relay && go test ./internal/anthropic/ -run TestRunSecurityWrapsStderr -v`
Expected: FAIL to compile (`runSecurity` undefined).

- [ ] **Step 3: Implement**

In `credentials.go`, extract the closure into a named function and use it in `LoadCredentials`:

```go
func runSecurity(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func LoadCredentials() (Credentials, error) {
	return loadCredentials(DefaultCredentialsPath(), runtime.GOOS, runSecurity)
}
```

Add `errors` and `strings` imports.

- [ ] **Step 4: Run full package tests with race detector**

Run: `cd relay && go test -race ./internal/anthropic/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add relay/internal/anthropic/
git commit -m "fix(relay): surface security(1) stderr in keychain credential errors"
```

---

### Task 5: Service install idempotency

**Files:**
- Modify: `relay/cmd/claude-pulse-relay/main.go`

**Interfaces:**
- Consumes: existing `runServiceCmd`.
- Produces: no signature changes. `service install` becomes safe to re-run: it unloads/stops any existing instance first (ignoring errors), then writes the file and loads.

Rationale: `launchctl load` on an already-loaded label fails ("service already loaded"), making a second `service install` fatal. Users will re-run install after upgrades.

- [ ] **Step 1: Implement**

In the `install` branch of `runServiceCmd`, before writing the file, add a best-effort unload:

```go
	// Best-effort stop of any existing instance so install is idempotent.
	if _, err := os.Stat(path); err == nil {
		var stop *exec.Cmd
		if runtime.GOOS == "darwin" {
			stop = exec.Command("launchctl", "unload", path)
		} else {
			stop = exec.Command("systemctl", "--user", "disable", "--now", "claude-pulse-relay")
		}
		stop.Run() // ignore errors: not loaded is fine
	}
```

This is OS-interaction glue with no seam for a unit test; verification is by build + (later, machine-local) manual re-run. Do not add a test that merely mocks `exec.Command` wiring.

- [ ] **Step 2: Build and run all tests**

Run: `cd relay && go build ./... && go test -race ./...`
Expected: clean build, all PASS.

- [ ] **Step 3: Commit**

```bash
git add relay/cmd/claude-pulse-relay/main.go
git commit -m "fix(relay): make service install idempotent by unloading existing instance"
```
