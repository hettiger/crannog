package hostfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

func TestOpenAnchor(t *testing.T) {
	tmp, dir := testfs.TempRoot(t)
	_, err := OpenAnchor(filepath.Join(dir, "home"), false)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	r, err := OpenAnchor(filepath.Join(dir, "home"), true)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	r, err = OpenAnchor(filepath.Join(dir, "home"), true)
	require.NoError(t, err, "existing")
	require.NoError(t, r.Close())
	assert.True(t, testfs.Exists(t, tmp, "home"))

	_, err = OpenAnchor(filepath.Join(dir, "a", "b"), true)
	assert.Error(t, err, "parents are not created")
}

func TestOpenDir(t *testing.T) {
	tmp, _ := testfs.TempRoot(t)
	testfs.WriteFile(t, tmp, "a/b/file", "x", 0o644)

	r, err := OpenDir(tmp, "a/b", false)
	require.NoError(t, err)
	assert.Equal(t, "x", testfs.ReadFile(t, r, "file"))
	require.NoError(t, r.Close())

	_, err = OpenDir(tmp, "a/missing", false)
	assert.ErrorIs(t, err, fs.ErrNotExist)
	r, err = OpenDir(tmp, "a/new/deeper", true)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	assert.True(t, testfs.Exists(t, tmp, "a/new/deeper"))

	r, err = OpenDir(tmp, ".", false)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	for _, name := range []string{"", "..", "a/../a", "a//b", "/a", "a/", "a/./b", "a/b/file"} {
		_, err := OpenDir(tmp, name, true)
		assert.Error(t, err, name)
	}
}

// No component may be a symlink, even one within the root.
func TestOpenDirRefusesSymlinks(t *testing.T) {
	tmp, dir := testfs.TempRoot(t)
	require.NoError(t, tmp.MkdirAll("root/a", 0o755))
	require.NoError(t, tmp.Mkdir("outside", 0o755))
	root, err := tmp.OpenRoot("root")
	require.NoError(t, err)
	defer root.Close()
	require.NoError(t, root.Symlink("a", "inside"))
	require.NoError(t, root.Symlink(filepath.Join(dir, "outside"), "a/outside"))

	for _, name := range []string{"inside", "inside/x", "a/outside", "a/outside/x"} {
		for _, create := range []bool{false, true} {
			_, err := OpenDir(root, name, create)
			assert.ErrorContains(t, err, "not a directory", name)
		}
	}
	entries, err := fs.ReadDir(tmp.FS(), "outside")
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing created through a symlink")
}

// A directory replaced by a symlink between the check and the open is
// refused, even if the symlink stays within the root.
func TestOpenDirRefusesAReplacedDirectory(t *testing.T) {
	root, _ := testfs.TempRoot(t)
	testfs.WriteFile(t, root, "a/file", "a", 0o644)
	testfs.WriteFile(t, root, "b/file", "b", 0o644)
	beforeOpen = func(dir *os.Root, name string) {
		require.NoError(t, dir.Rename(name, name+"-moved"))
		require.NoError(t, dir.Symlink("b", name))
	}
	t.Cleanup(func() { beforeOpen = func(*os.Root, string) {} })

	_, err := OpenDir(root, "a", false)
	assert.ErrorContains(t, err, "replaced")
}

func TestMkdirTemp(t *testing.T) {
	tmp, _ := testfs.TempRoot(t)
	a, err := MkdirTemp(tmp, ".tmp-")
	require.NoError(t, err)
	b, err := MkdirTemp(tmp, ".tmp-")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.True(t, strings.HasPrefix(a, ".tmp-"))
	info, err := tmp.Stat(a)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestTempDir(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	tmp, err := NewTempDir("diff")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(filepath.Base(tmp.Dir), "crannog-diff-"), tmp.Dir)
	assert.Equal(t, os.TempDir(), filepath.Dir(tmp.Dir))
	info, err := tmp.Stat(".")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	testfs.WriteFile(t, tmp.Root, "a/b", "x", 0o644)

	require.NoError(t, tmp.Remove())
	entries, err := fs.ReadDir(testfs.Root(t, os.TempDir()).FS(), ".")
	require.NoError(t, err)
	assert.Empty(t, entries, "removed")
}

// Remove deletes in the directory the temporary directory was created in,
// even if the path of the user's temporary directory leads elsewhere by
// then.
func TestTempDirRemoveStaysInItsParent(t *testing.T) {
	outer, dir := testfs.TempRoot(t)
	require.NoError(t, outer.Mkdir("tmp", 0o700))
	t.Setenv("TMPDIR", filepath.Join(dir, "tmp"))
	tmp, err := NewTempDir("diff")
	require.NoError(t, err)
	name := filepath.Base(tmp.Dir)

	require.NoError(t, outer.Rename("tmp", "tmp-moved"))
	testfs.WriteFile(t, outer, "tmp/"+name+"/keep", "keep", 0o644)

	require.NoError(t, tmp.Remove())
	assert.Equal(t, "keep", testfs.ReadFile(t, outer, "tmp/"+name+"/keep"), "the other directory is untouched")
	assert.False(t, testfs.Exists(t, outer, "tmp-moved/"+name), "removed where it was created")
}
