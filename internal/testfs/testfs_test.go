package testfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTB records a fatal failure instead of failing the test.
type fakeTB struct {
	testing.TB
	failed   bool
	cleanups []func()
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatal(...any) { f.failed = true; runtime.Goexit() }

func (f *fakeTB) Fatalf(string, ...any) { f.failed = true; runtime.Goexit() }

func (f *fakeTB) Cleanup(fn func()) { f.cleanups = append(f.cleanups, fn) }

// fails reports whether fn fails fatally.
func fails(t *testing.T, fn func(tb testing.TB)) bool {
	f := &fakeTB{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(f)
	}()
	<-done
	for _, c := range f.cleanups {
		c()
	}
	return f.failed
}

func TestRootAcceptsTemporaryDirectories(t *testing.T) {
	_, dir := TempRoot(t)
	assert.False(t, fails(t, func(tb testing.TB) { Root(tb, dir) }))
}

func TestRootRefusesOtherDirectories(t *testing.T) {
	_, dir := TempRoot(t)
	for _, other := range []string{"/", os.TempDir(), filepath.Join(dir, "..", "..")} {
		assert.True(t, fails(t, func(tb testing.TB) { Root(tb, other) }), other)
	}
}

// A symlink is resolved before the check and the root is opened on its
// target.
func TestRootResolvesSymlinks(t *testing.T) {
	root, dir := TempRoot(t)
	_, target := TempRoot(t)
	require.NoError(t, root.Symlink(target, "inside"))
	require.NoError(t, root.Symlink("/", "outside"))

	assert.Equal(t, target, Root(t, filepath.Join(dir, "inside")).Name())
	assert.True(t, fails(t, func(tb testing.TB) { Root(tb, filepath.Join(dir, "outside")) }))
}

// A test that changes $TMPDIR does not move the boundary.
func TestRootIgnoresChangedTMPDIR(t *testing.T) {
	t.Setenv("TMPDIR", "/")
	assert.True(t, fails(t, func(tb testing.TB) { Root(tb, "/") }))
}

func TestRepo(t *testing.T) {
	repo := Repo(t)
	mod, err := fs.ReadFile(repo, "go.mod")
	require.NoError(t, err)
	assert.True(t, declares(mod, module))
	_, err = fs.ReadFile(repo, "../go.mod")
	assert.Error(t, err, "stays within the repository")
}

func TestDeclares(t *testing.T) {
	assert.True(t, declares([]byte("// comment\nmodule "+module+"\n\ngo 1.27\n"), module))
	assert.False(t, declares([]byte("module example.com/other\n"), module))
	assert.False(t, declares([]byte("go 1.27\n"), module))
}
