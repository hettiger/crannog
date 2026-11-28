// Package zed runs zed, the command line tool of the Zed editor. It is
// the one place crannog starts zed (ADR 0031).
package zed

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/hettiger/crannog/internal/hostfs"
)

// program is the name of Zed's command line tool.
const program = "zed"

// CLI runs zed from PATH.
type CLI struct{}

// lookPath returns the path of zed in PATH. In tests it must be a fake
// below the temporary directory: the real one would open windows.
func lookPath() (string, error) {
	p, err := exec.LookPath(program)
	if err != nil {
		return "", errors.New("zed is not in PATH; install Zed's command line tool in Zed with Zed > Install CLI")
	}
	if testing.Testing() {
		ok, err := hostfs.BelowTestTempDir(p)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("tests must run a fake zed in a temporary directory, not %s", p)
		}
	}
	return p, nil
}

// OpenSSH opens the directory dir on the SSH host host in Zed, which
// connects with the system ssh. zed returns once Zed has the request.
func (CLI) OpenSSH(host, dir string) error {
	if host == "" || strings.ContainsAny(host, "/@:") {
		return fmt.Errorf("invalid SSH host %q", host)
	}
	if !path.IsAbs(dir) {
		return fmt.Errorf("not an absolute path: %q", dir)
	}
	p, err := lookPath()
	if err != nil {
		return err
	}
	u := sshURL(host, dir)
	out, err := exec.Command(p, u).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("zed %s: %s", u, msg)
		}
		return fmt.Errorf("zed %s: %w", u, err)
	}
	return nil
}

// sshURL returns the ssh:// URL of dir on host. Zed decodes the path, so
// a space or # in it is escaped.
func sshURL(host, dir string) string {
	return (&url.URL{Scheme: "ssh", Host: host, Path: dir}).String()
}
