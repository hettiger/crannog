package seatbelt

import (
	"fmt"
	"os"
	"syscall"

	"github.com/hettiger/crannog/internal/seatbelt/selftest"
)

// Enter confines the process. Called first in main, it replaces the
// process with sandbox-exec, which runs the same binary with the same
// arguments in Seatbelt; there, Enter runs the self-test and returns. It
// returns nil only in a process that cannot write outside of the profile.
//
// Seatbelt refuses to enter from inside another sandbox: sandbox-exec
// then fails with "sandbox_apply: Operation not permitted".
func Enter() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find the home directory: %w", err)
	}
	if os.Getenv(Env) != "" {
		return selftest.Run(home)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	l, err := locate()
	if err != nil {
		return err
	}
	err = syscall.Exec(sandboxExec, command(exe, os.Args[1:], l), append(os.Environ(), Env+"=1"))
	return fmt.Errorf("cannot run %s: %w", sandboxExec, err)
}
