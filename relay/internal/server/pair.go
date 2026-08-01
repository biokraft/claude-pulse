package server

import (
	"html/template"
	"log"
	"net/http"
	"strings"
)

// The pairing page is what the QR code on the terminal points at. Scanning it
// on a phone should answer one question — what do I type into Garmin Connect —
// so the page shows the relay URL and the token, each with a copy button, and
// nothing else. It is guarded by the same token as the API: the QR URL carries
// it, so anyone who can scan the code can read the page.
var pairTemplate = template.Must(template.New("pair").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="dark light">
<title>Claude Pulse — pairing</title>
<style>
  :root { color-scheme: dark light; }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 2rem 1.25rem 3rem;
    font: 16px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
    background: #141312; color: #f7f5f2;
    -webkit-text-size-adjust: 100%;
  }
  main { max-width: 34rem; margin: 0 auto; }
  h1 { font-size: 1.4rem; margin: 0 0 .25rem; }
  p.sub { margin: 0 0 2rem; color: #a9a399; }
  h2 { font-size: .8rem; text-transform: uppercase; letter-spacing: .08em;
       color: #a9a399; margin: 1.75rem 0 .5rem; font-weight: 600; }
  .field { display: flex; gap: .5rem; align-items: stretch; }
  .value {
    flex: 1; min-width: 0; padding: .75rem .85rem;
    background: #1e1c1a; border: 1px solid #2e2e2e; border-radius: .6rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: .95rem;
    overflow-wrap: anywhere; user-select: all;
  }
  button {
    flex: 0 0 auto; padding: 0 1rem; min-height: 100%;
    background: #cc7a56; color: #141312; font: inherit; font-weight: 600;
    border: 0; border-radius: .6rem; cursor: pointer;
  }
  button:active { opacity: .8; }
  button[data-done="1"] { background: #6f9a6a; }
  ol { padding-left: 1.2rem; color: #d6d1c9; }
  li { margin: .4rem 0; }
  code { background: #1e1c1a; padding: .1rem .35rem; border-radius: .3rem;
         font-size: .9em; }
  footer { margin-top: 2.5rem; color: #8a857c; font-size: .85rem; }
  a { color: #cc7a56; }
</style>
</head>
<body>
<main>
  <h1>Claude Pulse</h1>
  <p class="sub">Relay is running. Enter these two values on your watch.</p>

  <h2>Relay URL</h2>
  <div class="field">
    <div class="value" id="url">{{.URL}}</div>
    <button type="button" data-copy="url">Copy</button>
  </div>

  <h2>Token</h2>
  <div class="field">
    <div class="value" id="token">{{.Token}}</div>
    <button type="button" data-copy="token">Copy</button>
  </div>

  <h2>Where they go</h2>
  <ol>
    <li>Open <strong>Garmin Connect</strong> on this phone.</li>
    <li>Go to <code>Connect IQ apps → Claude Pulse → Settings</code>.</li>
    <li>Paste the URL and token, then save.</li>
  </ol>

  <footer>
    Quick-tunnel URLs change every time the relay restarts. Run
    <code>claude-pulse-relay service install</code> to keep it up, or check
    <a href="https://github.com/biokraft/claude-pulse">the docs</a>.
  </footer>
</main>
<script>
document.querySelectorAll("button[data-copy]").forEach(function (btn) {
  btn.addEventListener("click", function () {
    var el = document.getElementById(btn.dataset.copy);
    var flash = function (label) {
      btn.textContent = label;
      btn.dataset.done = "1";
      setTimeout(function () { btn.textContent = "Copy"; btn.dataset.done = ""; }, 1500);
    };
    // navigator.clipboard only exists in a secure context, so it is missing
    // over plain http (--no-tunnel). Fall back to selecting the text, which
    // leaves the phone one long-press away from copying it.
    var select = function () {
      var range = document.createRange();
      range.selectNodeContents(el);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      flash("Selected");
    };
    if (navigator.clipboard) {
      navigator.clipboard.writeText(el.textContent).then(function () {
        flash("Copied");
      }, select);
    } else {
      select();
    }
  });
});
</script>
</body>
</html>
`))

// unauthorizedHTML is what a browser gets for a missing or stale token —
// almost always an old QR code, since quick-tunnel URLs rotate on restart.
const unauthorizedHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="dark light">
<title>Claude Pulse — not authorized</title>
<style>
  body { margin: 0; padding: 3rem 1.5rem;
         font: 16px/1.6 -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
         background: #141312; color: #f7f5f2; }
  main { max-width: 30rem; margin: 0 auto; }
  h1 { font-size: 1.3rem; margin: 0 0 .75rem; }
  p { color: #a9a399; }
  code { background: #1e1c1a; padding: .1rem .35rem; border-radius: .3rem; }
</style>
</head>
<body><main>
  <h1>Not authorized</h1>
  <p>This link is missing a valid token. Quick-tunnel URLs and QR codes are
  only good until the relay restarts — scan the newest QR code printed in the
  relay's terminal, or run <code>claude-pulse-relay</code> again to print one.</p>
</main></body>
</html>
`

// publicURL reconstructs the address the client used, so the page shows the
// tunnel URL rather than the relay's local listen address. Cloudflare sets
// X-Forwarded-Proto; a direct hit on the local port has neither header.
func publicURL(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// PairHandler renders the pairing page. Only "/" is the page; every other
// unmatched path is a genuine 404, which the default mux would otherwise
// swallow into this handler.
func PairHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if err := pairTemplate.Execute(w, struct{ URL, Token string }{
			URL:   publicURL(r),
			Token: token,
		}); err != nil {
			log.Printf("pair page: %v", err)
		}
	})
}
