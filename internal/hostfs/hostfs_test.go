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

func TestOpenCrannogHome(t *testing.T) {
	tmp, dir := testfs.TempRoot(t)
	t.Setenv("HOME", dir)
	r, err := OpenCrannogHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".crannog"), r.Name())
	info, err := r.Stat(".")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "the user's only")
	require.NoError(t, r.Close())
	r, err = OpenCrannogHome()
	require.NoError(t, err, "existing")
	require.NoError(t, r.Close())
	assert.Equal(t, []string{".crannog"}, names(t, tmp, "."), "nothing else in the home directory")

	t.Setenv("HOME", filepath.Join(dir, "missing"))
	_, err = OpenCrannogHome()
	assert.Error(t, err, "the home directory is not created")
}

// Tests cannot open the crannog home outside of the temporary directory,
// e.g. the user's real one. The package directory in the repository stands
// in for it.
func TestOpenCrannogHomeRefusesHomeOutsideTemp(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	if checkTestHome(wd) == nil {
		t.Skip("the repository is in the temporary directory")
	}
	t.Setenv("HOME", wd)
	_, err = OpenCrannogHome()
	assert.ErrorContains(t, err, "tests must set HOME to a temporary directory")
	_, err = fs.Stat(testfs.Repo(t), "internal/hostfs/.crannog")
	assert.ErrorIs(t, err, fs.ErrNotExist, "nothing created")
}

// names returns the sorted names in the directory name within root.
func names(t *testing.T, root *os.Root, name string) []string {
	entries, err := fs.ReadDir(root.FS(), name)
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestOpenWorktree(t *testing.T) {
	tmp, dir := testfs.TempRoot(t)
	_, err := OpenWorktree(dir)
	assert.ErrorContains(t, err, "not the top of a git worktree")

	for _, git := range []string{"dir", "file"} {
		t.Run(git, func(t *testing.T) {
			tmp, dir := testfs.TempRoot(t)
			if git == "dir" {
				require.NoError(t, tmp.Mkdir(".git", 0o755))
			} else {
				testfs.WriteFile(t, tmp, ".git", "gitdir: elsewhere\n", 0o644)
			}
			wt, err := OpenWorktree(dir)
			require.NoError(t, err)
			require.NoError(t, wt.Close())
		})
	}
	assert.Empty(t, names(t, tmp, "."), "nothing created")
}

func tempWorktree(t *testing.T) (*os.Root, *Worktree) {
	tmp, dir := testfs.TempRoot(t)
	require.NoError(t, tmp.Mkdir(".git", 0o755))
	wt, err := OpenWorktree(dir)
	require.NoError(t, err)
	t.Cleanup(func() { wt.Close() })
	return tmp, wt
}

func TestWorktreeSetup(t *testing.T) {
	tmp, wt := tempWorktree(t)
	_, err := wt.LstatSetup()
	assert.ErrorIs(t, err, fs.ErrNotExist)
	_, err = wt.OpenSetup()
	assert.ErrorIs(t, err, fs.ErrNotExist)

	testfs.WriteFile(t, tmp, ".crannog/config.yml", "x", 0o644)
	info, err := wt.LstatSetup()
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	r, err := wt.OpenSetup()
	require.NoError(t, err)
	assert.Equal(t, "x", testfs.ReadFile(t, r, "config.yml"))
	require.NoError(t, r.Close())

	require.NoError(t, tmp.Rename(".crannog", "moved"))
	require.NoError(t, tmp.Symlink("moved", ".crannog"))
	info, err = wt.LstatSetup()
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&fs.ModeSymlink, "a symlink is not followed")
	_, err = wt.OpenSetup()
	assert.ErrorContains(t, err, "not a directory")
}

// A setup temp replaces .crannog by moves within the worktree and only
// touches .crannog and itself.
func TestSetupTemp(t *testing.T) {
	tmp, wt := tempWorktree(t)
	testfs.WriteFile(t, tmp, ".crannog/old.txt", "old", 0o644)
	testfs.WriteFile(t, tmp, "keep.txt", "keep", 0o644)

	st, err := wt.NewSetupTemp()
	require.NoError(t, err)
	name := filepath.Base(st.Dir)
	assert.True(t, strings.HasPrefix(name, ".crannog.tmp-"), name)
	testfs.WriteFile(t, st.Root, "next/new.txt", "new", 0o644)

	require.NoError(t, st.Stash("old"))
	assert.Equal(t, "old", testfs.ReadFile(t, st.Root, "old/old.txt"))
	require.NoError(t, st.Install("next"))
	assert.Equal(t, "new", testfs.ReadFile(t, tmp, ".crannog/new.txt"))
	for _, bad := range []string{"", ".", "..", "a/b", "../keep.txt"} {
		assert.Error(t, st.Stash(bad), bad)
		assert.Error(t, st.Install(bad), bad)
	}

	require.NoError(t, st.Remove())
	assert.Equal(t, []string{".crannog", ".git", "keep.txt"}, names(t, tmp, "."))
	assert.Equal(t, "keep", testfs.ReadFile(t, tmp, "keep.txt"))
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
	testfs.WriteFile(t, outer, filepath.Join("tmp", name, "keep"), "keep", 0o644)

	require.NoError(t, tmp.Remove())
	assert.Equal(t, "keep", testfs.ReadFile(t, outer, filepath.Join("tmp", name, "keep")), "the other directory is untouched")
	assert.False(t, testfs.Exists(t, outer, filepath.Join("tmp-moved", name)), "removed where it was created")
}
