package tunnel

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	line := "2026-07-27T10:00:00Z INF |  https://tall-cactus-abc123.trycloudflare.com  |"
	url, ok := ParseURL(line)
	if !ok || url != "https://tall-cactus-abc123.trycloudflare.com" {
		t.Fatalf("got %q %v", url, ok)
	}
	if _, ok := ParseURL("random log line"); ok {
		t.Fatal("false positive")
	}
}

func TestPrintPairingIncludesURLAndToken(t *testing.T) {
	var buf bytes.Buffer
	PrintPairing(&buf, "https://x.trycloudflare.com", "sekret")
	out := buf.String()
	if !strings.Contains(out, "https://x.trycloudflare.com") || !strings.Contains(out, "sekret") {
		t.Fatalf("missing pairing info:\n%s", out)
	}
}
