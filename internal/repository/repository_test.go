package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testgit"
)

func TestOpen(t *testing.T) {
	dir := testgit.Init(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	r, err := Open(filepath.Join(dir, "sub"))
	require.NoError(t, err)
	assert.Equal(t, dir, r.Dir)
	assert.Equal(t, dir, r.MainWorktree)
	assert.Equal(t, filepath.Join(dir, ".git"), r.CommonDir)
	assert.Equal(t, ID(r.CommonDir), r.ID)
	assert.Len(t, r.ID, 64)
	assert.Equal(t, r.ID[:8], r.ID8())
	assert.Equal(t, "/work"+dir, r.PathInCrannog())
}

func TestOpenFromWorktree(t *testing.T) {
	dir := testgit.Init(t)
	wt := filepath.Join(filepath.Dir(dir), "wt")
	testgit.Git(t, dir, "worktree", "add", "-q", "-b", "other", wt)

	main, err := Open(dir)
	require.NoError(t, err)
	r, err := Open(wt)
	require.NoError(t, err)
	assert.Equal(t, wt, r.Dir)
	assert.Equal(t, dir, r.MainWorktree, "main worktree")
	assert.Equal(t, main.ID, r.ID, "same id for all worktrees")

	b, err := r.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "other", b)
}

func TestOpenOutsideRepository(t *testing.T) {
	testgit.Isolate(t)
	_, err := Open(t.TempDir())
	assert.Error(t, err)
}

func TestLocateWithGitDir(t *testing.T) {
	dir := testgit.Init(t)
	t.Setenv("GIT_DIR", filepath.Join(dir, ".git"))
	r, err := Locate(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, dir, r.MainWorktree)
	assert.Equal(t, ID(filepath.Join(dir, ".git")), r.ID)
}

func TestStartBranch(t *testing.T) {
	dir := testgit.Init(t)
	testgit.Git(t, dir, "branch", "feature/x")
	r, err := Open(dir)
	require.NoError(t, err)

	b, err := r.StartBranch("")
	require.NoError(t, err)
	assert.Equal(t, "main", b)

	b, err = r.StartBranch("feature/x")
	require.NoError(t, err)
	assert.Equal(t, "feature/x", b)

	_, err = r.StartBranch("missing")
	assert.ErrorContains(t, err, "not a local branch")

	testgit.Git(t, dir, "switch", "-q", "--detach")
	_, err = r.StartBranch("")
	assert.ErrorIs(t, err, ErrDetached)
	b, err = r.StartBranch("main")
	require.NoError(t, err)
	assert.Equal(t, "main", b)
}

func TestMainWorktree(t *testing.T) {
	_, err := mainWorktree("worktree /x\nbare\n")
	assert.ErrorContains(t, err, "bare")

	p, err := mainWorktree("worktree /a b\nHEAD 123\nbranch refs/heads/main\n\nworktree /c\n")
	require.NoError(t, err)
	assert.Equal(t, "/a b", p)
}
