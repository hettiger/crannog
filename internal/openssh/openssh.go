// Package openssh runs ssh, the OpenSSH client of the host, with the
// user's SSH configuration. It is the one place crannog starts ssh
// (ADR 0031).
package openssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/hettiger/crannog/internal/hostfs"
)

// program is the name of the OpenSSH client.
const program = "ssh"

// Client runs ssh from PATH.
type Client struct{}

// Options of a connection.
type Options struct {
	// Terminal asks for a terminal in the crannog (-t); without it, ssh
	// allocates none (-T).
	Terminal bool
	// Command is what the remote user's shell runs, as one string.
	Command string
}

// path returns the path of ssh in PATH. In tests it must be a fake below
// the temporary directory: the real one would connect to the user's
// hosts.
func path() (string, error) {
	p, err := exec.LookPath(program)
	if err != nil {
		return "", errors.New("ssh is not in PATH")
	}
	if testing.Testing() {
		ok, err := hostfs.BelowTestTempDir(p)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("tests must run a fake ssh in a temporary directory, not %s", p)
		}
	}
	return p, nil
}

// checkHost refuses a host name that ssh would read as an option.
func checkHost(host string) error {
	if host == "" || strings.HasPrefix(host, "-") {
		return fmt.Errorf("invalid SSH host %q", host)
	}
	return nil
}

// Config returns the options ssh would use to connect to host, as
// ssh -G prints them: by option name in lower case, the values in order.
// It connects to nothing.
func (Client) Config(ctx context.Context, host string) (map[string][]string, error) {
	if err := checkHost(host); err != nil {
		return nil, err
	}
	p, err := path()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, p, "-G", host)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("ssh -G %s: %s", host, msg)
		}
		return nil, fmt.Errorf("ssh -G %s: %w", host, err)
	}
	return parseConfig(out), nil
}

// parseConfig reads the output of ssh -G: one option per line, its name
// and value separated by a space.
func parseConfig(out []byte) map[string][]string {
	c := map[string][]string{}
	for line := range strings.Lines(string(out)) {
		k, v, ok := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		if ok {
			c[k] = append(c[k], v)
		}
	}
	return c
}

// Replace replaces this process with ssh to host, with the standard input
// and outputs and the environment of this process. It only returns on
// errors.
func (Client) Replace(host string, o Options) error {
	if err := checkHost(host); err != nil {
		return err
	}
	p, err := path()
	if err != nil {
		return err
	}
	err = syscall.Exec(p, connectArgs(host, o), os.Environ())
	return fmt.Errorf("cannot run %s: %w", p, err)
}

// connectArgs returns the arguments of ssh, its name first. ControlPath
// none: a control socket that the user's configuration puts in ~/.ssh
// cannot be created in Seatbelt (ADR 0030), and the connection needs none.
func connectArgs(host string, o Options) []string {
	a := []string{program, "-o", "ControlPath=none"}
	if o.Terminal {
		a = append(a, "-t")
	} else {
		a = append(a, "-T")
	}
	a = append(a, host)
	if o.Command != "" {
		a = append(a, o.Command)
	}
	return a
}
