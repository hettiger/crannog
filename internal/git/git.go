// Package git runs the user's git on the host.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Command returns a git command that runs in dir.
func Command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd
}

// Run runs git in dir and returns its stdout without the trailing newline.
// On failure, the error contains git's stderr.
func Run(dir string, args ...string) (string, error) {
	out, err := Output(dir, args...)
	return strings.TrimRight(string(out), "\n"), err
}

// Output runs git in dir and returns its stdout unchanged.
func Output(dir string, args ...string) ([]byte, error) {
	cmd := Command(dir, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return out, nil
}

// Error is a failed git command.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode returns the exit code of a failed git command, or -1.
func ExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
