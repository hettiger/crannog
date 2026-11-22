// Package selftest checks that Seatbelt is active (ADR 0030): creating a
// file where the profile allows no writes must fail. The binaries run it
// after entering Seatbelt. On macOS every test binary runs it when it
// starts, by linking this package, so tests do not run outside of
// Seatbelt.
package selftest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// CheckFile is the file the self-test tries to create.
const CheckFile = ".crannog-sandbox-check"

// Run checks that Seatbelt is active: creating CheckFile in dir, the
// user's home directory, must fail because the operation is not
// permitted. If the file can be created, it is deleted again and the
// check fails. Any other error fails it too: a file that exists or a
// directory that cannot be written to says nothing about Seatbelt.
func Run(dir string) error {
	name := filepath.Join(dir, CheckFile)
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		return errors.Join(
			fmt.Errorf("Seatbelt is not active: %s could be created", name),
			f.Close(), os.Remove(name))
	}
	if errors.Is(err, syscall.EPERM) {
		return nil
	}
	return fmt.Errorf("cannot check that Seatbelt is active: %w", err)
}

// init stops a test binary on macOS before any test runs unless it runs
// in Seatbelt, so tests started without it fail instead of running
// unconfined. Every test binary links this package; a test in it checks
// that. The self-test runs in the user's real home directory, before any
// test can change $HOME. In the binaries init does nothing: they run the
// self-test after entering Seatbelt.
func init() {
	if !testing.Testing() {
		return
	}
	home, err := os.UserHomeDir()
	if err := tests(runtime.GOOS, home, err); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func tests(goos, home string, homeErr error) error {
	if goos != "darwin" {
		return nil
	}
	if homeErr != nil {
		return homeErr
	}
	if err := Run(home); err != nil {
		return fmt.Errorf("on macOS, tests must run in Seatbelt (make test): %w", err)
	}
	return nil
}
