// Package selftest checks that Seatbelt is active (ADR 0030): creating a
// file where the profile allows no writes must fail. The binaries run it
// after entering Seatbelt, and on macOS the tests' file access runs it so
// tests do not run outside of Seatbelt.
package selftest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
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

// home is the user's home directory, taken once in a test binary before
// any test can change $HOME.
var home, homeErr = testHome()

func testHome() (string, error) {
	if !testing.Testing() {
		return "", nil
	}
	return os.UserHomeDir()
}

var inTests = sync.OnceValue(func() error { return tests(runtime.GOOS, home, homeErr) })

// InTests fails in a test binary on macOS that does not run in Seatbelt.
// Everything that gives tests access to files calls it, so tests started
// without Seatbelt fail instead of running unconfined. It runs the
// self-test once, in the user's real home directory.
func InTests() error {
	if !testing.Testing() {
		return nil
	}
	return inTests()
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
