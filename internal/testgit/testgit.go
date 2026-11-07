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
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(dir, "repo")
	Git(t, "", "init", "-q", "-b", "main", dir)
	Git(t, dir, "commit", "-q", "--allow-empty", "-m", "Initial commit")
	return dir
}

// Git runs git in dir and returns its trimmed output.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// WriteFile writes a file below dir, creating parent directories.
func WriteFile(t *testing.T, dir, name, content string, mode os.FileMode) {
	t.Helper()
	testfs.WriteFile(t, testfs.Root(t, dir), filepath.ToSlash(name), content, mode)
}
