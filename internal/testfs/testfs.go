// Package testfs gives tests a root to work in, so tests access the file
// system like crannog does (ADR 0032).
package testfs

import (
	"os"
	"path/filepath"
	"testing"
)

// Root opens dir as a root that is closed at the end of the test.
func Root(t testing.TB, dir string) *os.Root {
	t.Helper()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// TempRoot returns a new temporary directory and a root for it.
func TempRoot(t testing.TB) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	return Root(t, dir), dir
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
