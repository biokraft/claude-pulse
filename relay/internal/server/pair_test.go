package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// getHTML mimics a phone browser following the QR code: it announces that it
// accepts HTML, which is what distinguishes a scan from the watch's API call.
func getHTML(t *testing.T, srv *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest("GET", srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(body)
}

// The QR code encodes exactly this URL, so a 404 here is a broken pairing flow.
func TestPairPageServedAtRootWithToken(t *testing.T) {
	srv := testHandler(t)
	resp, body := getHTML(t, srv, "/?token=sekret")

	if resp.StatusCode != 200 {
		t.Fatalf("code %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type %q, want text/html — a phone downloads anything else", ct)
	}
	if !strings.Contains(body, "sekret") {
		t.Error("page does not show the token")
	}
	if !strings.Contains(body, srv.Listener.Addr().String()) {
		t.Errorf("page does not show the relay host %q", srv.Listener.Addr().String())
	}
}

func TestPairPageRequiresToken(t *testing.T) {
	srv := testHandler(t)

	for _, path := range []string{"/", "/?token=wrong"} {
		resp, body := getHTML(t, srv, path)
		if resp.StatusCode != 401 {
			t.Fatalf("%s: code %d, want 401", path, resp.StatusCode)
		}
		if strings.Contains(body, "sekret") {
			t.Fatalf("%s: leaked the token to an unauthorized client", path)
		}
		// Plain text with nosniff is what made Safari download the old 404.
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: content-type %q, want text/html", path, ct)
		}
	}
}

func TestUnknownPathIs404(t *testing.T) {
	srv := testHandler(t)
	resp, _ := getHTML(t, srv, "/nope?token=sekret")
	if resp.StatusCode != 404 {
		t.Fatalf("code %d, want 404", resp.StatusCode)
	}
}

func TestPublicURLPrefersForwardedProto(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"direct", nil, "http://relay.local"},
		{"tunnel", map[string]string{"X-Forwarded-Proto": "https"}, "https://relay.local"},
		{"proxy chain", map[string]string{"X-Forwarded-Proto": "https, http"}, "https://relay.local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://relay.local/", nil)
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}
			if got := publicURL(r); got != tc.want {
				t.Errorf("publicURL = %q, want %q", got, tc.want)
			}
		})
	}
}
