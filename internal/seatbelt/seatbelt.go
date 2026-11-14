// Package seatbelt confines crannog and git-remote-crannog with the macOS
// sandbox, Seatbelt (ADR 0030): the binaries run themselves again through
// sandbox-exec with a profile that allows writing files only where
// crannog has to. The kernel enforces it for the process and everything
// it runs.
package seatbelt

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/repository"
)

// Env marks a process that was started through sandbox-exec. It only
// saves entering again, which Seatbelt refuses; the self-test decides
// whether Seatbelt is active.
const Env = "CRANNOG_SEATBELT"

// CheckFile is the file in the user's home directory the self-test tries
// to create.
const CheckFile = ".crannog-sandbox-check"

// sandboxExec is given as a path: it must not be found through PATH.
const sandboxExec = "/usr/bin/sandbox-exec"

// builderDir is where container build writes the build context, relative
// to the user's home directory.
var builderDir = filepath.Join("Library", "Application Support", "com.apple.container", "builder")

// none stands for a location that does not apply, such as the worktree
// outside of a repository. Every parameter of the profile needs a value;
// this one allows nothing the profile does not allow anyway.
const none = "/dev/null"

//go:embed profile.sb
var profile string

// locate returns the locations crannog writes to, for the repository of
// the working directory if there is one.
func locate() (hostfs.Locations, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return hostfs.Locations{}, err
	}
	var worktree, gitDir string
	if r, err := repository.Open(cwd); err == nil {
		worktree, gitDir = r.Dir, r.CommonDir
	} else if r, err := repository.Locate(cwd); err == nil {
		gitDir = r.CommonDir
	}
	return hostfs.Locate(worktree, gitDir)
}

// command returns the arguments that run exe with args through
// sandbox-exec, with the profile and its parameters.
func command(exe string, args []string, l hostfs.Locations) []string {
	params := []struct{ name, value string }{
		{"CRANNOG_HOME", l.CrannogHome},
		{"TEMP_DIR", l.TempDir},
		{"WORKTREE", l.Worktree},
		{"GIT_DIR", l.GitDir},
		{"BUILDER_DIR", filepath.Join(l.Home, builderDir)},
	}
	argv := []string{sandboxExec, "-p", profile}
	for _, p := range params {
		if p.value == "" {
			p.value = none
		}
		argv = append(argv, "-D", p.name+"="+p.value)
	}
	return append(append(argv, exe), args...)
}

// selfTest checks that Seatbelt is active: creating CheckFile in home
// must fail because the operation is not permitted. If the file can be
// created, it is deleted again and the check fails. Any other error
// fails it too: a file that exists or a home that cannot be written to
// says nothing about Seatbelt.
func selfTest(home string) error {
	name := filepath.Join(home, CheckFile)
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
