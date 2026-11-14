// Package testfs gives tests a root to work in, so tests access the file
// system like crannog does (ADR 0013). Tests write only below the user's
// temporary directory and read only the crannog repository outside of it.
package testfs

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const module = "github.com/hettiger/crannog"

// tempDir is the user's temporary directory with symlinks resolved, taken
// once before any test can change $TMPDIR.
var tempDir, tempDirErr = filepath.EvalSymlinks(os.TempDir())

// Root opens dir as a root that is closed at the end of the test. dir must
// be below the user's temporary directory, e.g. from t.TempDir(); the root
// is opened on dir with symlinks resolved.
func Root(t testing.TB, dir string) *os.Root {
	t.Helper()
	r, _ := tempRoot(t, dir)
	return r
}

// TempRoot returns a new temporary directory with symlinks resolved and a
// root for it.
func TempRoot(t testing.TB) (*os.Root, string) {
	t.Helper()
	return tempRoot(t, t.TempDir())
}

func tempRoot(t testing.TB, dir string) (*os.Root, string) {
	t.Helper()
	if tempDirErr != nil {
		t.Fatal(tempDirErr)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(tempDir, resolved)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		t.Fatalf("%s is not below the temporary directory %s", dir, tempDir)
	}
	return open(t, resolved), resolved
}

// Repo returns the crannog repository this package is in, read-only.
func Repo(t testing.TB) fs.FS {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(file) {
		t.Fatal("cannot locate the crannog repository")
	}
	r := open(t, filepath.Join(filepath.Dir(file), "..", ".."))
	mod, err := r.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if !declares(mod, module) {
		t.Fatalf("%s is not the repository of %s", r.Name(), module)
	}
	return r.FS()
}

// declares reports whether the go.mod content mod declares module.
func declares(mod []byte, module string) bool {
	s := bufio.NewScanner(bytes.NewReader(mod))
	for s.Scan() {
		if f := strings.Fields(s.Text()); len(f) == 2 && f[0] == "module" {
			return f[1] == module
		}
	}
	return false
}

func open(t testing.TB, dir string) *os.Root {
	t.Helper()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// WriteFile writes the file name within root with exactly mode, creating
// parent directories.
func WriteFile(t testing.TB, root *os.Root, name, content string, mode os.FileMode) {
	t.Helper()
	if dir := filepath.Dir(name); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := root.WriteFile(name, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := root.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
}

// ReadFile returns the content of the file name within root.
func ReadFile(t testing.TB, root *os.Root, name string) string {
	t.Helper()
	data, err := root.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Exists reports whether name exists within root, without following a
// final symlink.
func Exists(t testing.TB, root *os.Root, name string) bool {
	t.Helper()
	_, err := root.Lstat(name)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}
