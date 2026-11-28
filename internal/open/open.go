// Package open runs open, the macOS program that opens URLs with their
// app. It is the one place crannog starts open (ADR 0031). It opens only
// the URLs crannog builds here, never one given by a caller.
package open

import (
	"fmt"
	"net/netip"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/hettiger/crannog/internal/hostfs"
)

// program is the path of open. Tests replace it with a fake.
var program = "/usr/bin/open"

// Opener opens URLs with open.
type Opener struct{}

// check returns the path of open. In tests it must be a fake below the
// temporary directory: the real one would open windows.
func check() (string, error) {
	if testing.Testing() {
		ok, err := hostfs.BelowTestTempDir(program)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("tests must run a fake open in a temporary directory, not %s", program)
		}
	}
	return program, nil
}

// run opens u. open returns once the app has the URL.
func run(u string) error {
	p, err := check()
	if err != nil {
		return err
	}
	out, err := exec.Command(p, u).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("cannot open %s: %s", u, msg)
	}
	return nil
}

// ZedSSH opens the directory dir on the SSH host host in Zed, which
// connects with the system ssh. It opens Zed's URL for it: Zed's own
// command line tool starts Zed in a way that fails in any sandbox, also
// in Seatbelt (ADR 0030).
func (Opener) ZedSSH(host, dir string) error {
	if host == "" || strings.ContainsAny(host, "/@:") {
		return fmt.Errorf("invalid SSH host %q", host)
	}
	if !path.IsAbs(dir) {
		return fmt.Errorf("not an absolute path: %q", dir)
	}
	u := zedSSHURL(host, dir)
	if err := run(u); err != nil {
		return fmt.Errorf("%w (is Zed installed?)", err)
	}
	return nil
}

// zedSSHURL returns Zed's URL for dir on host, zed://ssh/<host><dir>.
// Zed decodes the path, so a space or # in it is escaped.
func zedSSHURL(host, dir string) string {
	return (&url.URL{Scheme: "zed", Host: "ssh", Path: "/" + host + dir}).String()
}

// Web opens WebURL(insecure, ip, port) in the default browser.
func (Opener) Web(insecure bool, ip netip.Addr, port uint16) error {
	if !ip.IsValid() {
		return fmt.Errorf("invalid address %s", ip)
	}
	return run(WebURL(insecure, ip, port))
}

// WebURL returns https://<ip>, or http:// if insecure, with :port unless
// port is 0.
func WebURL(insecure bool, ip netip.Addr, port uint16) string {
	u := url.URL{Scheme: "https", Host: ip.String()}
	if insecure {
		u.Scheme = "http"
	}
	if ip.Is6() {
		u.Host = "[" + u.Host + "]"
	}
	if port != 0 {
		u.Host += ":" + strconv.Itoa(int(port))
	}
	return u.String()
}
