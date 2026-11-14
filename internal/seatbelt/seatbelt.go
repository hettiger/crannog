// Package seatbelt confines crannog and git-remote-crannog with the macOS
// sandbox, Seatbelt (ADR 0030): the binaries run themselves again through
// sandbox-exec with a profile that allows writing files only where
// crannog has to. The kernel enforces it for the process and everything
// it runs.
package seatbelt

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/repository"
)

// Env marks a process that was started through sandbox-exec. It only
// saves entering again, which Seatbelt refuses; the self-test decides
// whether Seatbelt is active.
const Env = "CRANNOG_SEATBELT"

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
