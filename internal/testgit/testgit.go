// Package testgit creates git repositories for tests.
package testgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hettiger/crannog/internal/testfs"
)

// Isolate makes git ignore the user's and the system's configuration for
// the rest of the test.
func Isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@crannog.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@crannog.invalid")
}

// Init creates a repository with one commit on main and returns its
// physical path.
func Init(t *testing.T) string {
	t.Helper()
	Isolate(t)
	_, parent := testfs.TempRoot(t)
	dir := filepath.Join(parent, "repo")
	Git(t, parent, "init", "-q", "-b", "main", "repo")
	Git(t, dir, "commit", "-q", "--allow-empty", "-m", "Initial commit")
	return dir
}

// Git runs git in dir and returns its trimmed output. It fails the test
// if git fails. See Try for where git may run.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := Try(t, dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// Try runs git in dir and returns its trimmed output and its error. The
// arguments are free, the place is not (ADR 0031): dir must be below the
// user's temporary directory, and the options that make git work
// elsewhere are refused.
func Try(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	// Fails the test unless dir is below the temporary directory.
	testfs.Root(t, dir)
	if o := elsewhere(args); o != "" {
		t.Fatalf("git %s: %s makes git work outside of %s", strings.Join(args, " "), o, dir)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// elsewhere returns the option among git's own options, the ones before
// the subcommand, that makes git work in another place than its
// directory, or "".
func elsewhere(args []string) string {
	for i := 0; i < len(args) && strings.HasPrefix(args[i], "-"); i++ {
		name, _, _ := strings.Cut(args[i], "=")
		switch name {
		case "-C", "--git-dir", "--work-tree":
			return name
		case "-c", "--config-env", "--exec-path", "--namespace", "--super-prefix":
			if !strings.Contains(args[i], "=") {
				// The value is the next argument.
				i++
			}
		}
	}
	return ""
}

// WriteFile writes a file below dir, creating parent directories.
func WriteFile(t *testing.T, dir, name, content string, mode os.FileMode) {
	t.Helper()
	testfs.WriteFile(t, testfs.Root(t, dir), filepath.ToSlash(name), content, mode)
}
