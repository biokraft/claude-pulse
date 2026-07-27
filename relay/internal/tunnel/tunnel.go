package tunnel

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

var urlRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type Tunnel struct {
	URL string
	cmd *exec.Cmd
}

func ParseURL(line string) (string, bool) {
	m := urlRe.FindString(line)
	return m, m != ""
}

func PrintPairing(out io.Writer, url, token string) {
	fmt.Fprintf(out, "\n=== Claude Pulse watch pairing ===\n")
	fmt.Fprintf(out, "Relay URL: %s\nToken:     %s\n", url, token)
	fmt.Fprintf(out, "Enter both in Garmin Connect > Claude Pulse settings.\n\n")
	if qr, err := qrcode.New(url+"?token="+token, qrcode.Medium); err == nil {
		fmt.Fprintln(out, qr.ToSmallString(false))
	}
}

func Start(localAddr string, token string, out io.Writer) (*Tunnel, error) {
	path, err := exec.LookPath("cloudflared")
	if err != nil {
		return nil, fmt.Errorf("cloudflared not found (install: brew install cloudflared): %w", err)
	}
	cmd := exec.Command(path, "tunnel", "--url", "http://"+localAddr)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	t := &Tunnel{cmd: cmd}
	urlCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if u, ok := ParseURL(sc.Text()); ok {
				select {
				case urlCh <- u:
				default:
				}
			}
		}
	}()
	select {
	case u := <-urlCh:
		t.URL = u
	case <-time.After(30 * time.Second):
		t.Stop()
		return nil, fmt.Errorf("timed out waiting for tunnel URL")
	}
	PrintPairing(out, t.URL, token)
	return t, nil
}

func (t *Tunnel) Stop() {
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
}
