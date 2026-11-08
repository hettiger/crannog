package setup

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
)

func sample() Setup {
	return Setup{
		"config.yml":           {Data: []byte("session:\n  command: zsh\n")},
		"iron-proxy.yaml":      {Data: []byte("proxy: {}\n")},
		"container/Dockerfile": {Data: []byte("FROM debian\n")},
		"container/run.sh":     {Data: []byte("#!/bin/sh\n"), Executable: true},
	}
}

func writeSample(t *testing.T, dir string) {
	for p, f := range sample() {
		mode := os.FileMode(0o644)
		if f.Executable {
			mode = 0o755
		}
		testgit.WriteFile(t, dir, filepath.Join(Dir, p), string(f.Data), mode)
	}
}

func TestHash(t *testing.T) {
	s := sample()
	h := s.Hash()
	assert.Len(t, h, 64)
	assert.Equal(t, h, sample().Hash(), "stable")

	x := sample()
	x["container/run.sh"] = File{Data: x["container/run.sh"].Data}
	assert.NotEqual(t, h, x.Hash(), "executable bit counts")

	renamed := sample()
	renamed["container/run2.sh"] = renamed["container/run.sh"]
	delete(renamed, "container/run.sh")
	assert.NotEqual(t, h, renamed.Hash(), "paths count")

	// Path and content boundaries cannot be shifted.
	a := Setup{"a": {Data: []byte("bc")}}
	b := Setup{"ab": {Data: []byte("c")}}
	assert.NotEqual(t, a.Hash(), b.Hash())
}

func TestImageName(t *testing.T) {
	s := sample()
	name := s.ImageName()
	assert.Regexp(t, `^crannog-[0-9a-f]{12}$`, name)

	other := sample()
	other["config.yml"] = File{Data: []byte("changed")}
	assert.Equal(t, name, other.ImageName(), "only container/ counts")

	other["container/Dockerfile"] = File{Data: []byte("FROM alpine\n")}
	assert.NotEqual(t, name, other.ImageName())
}

func TestEqualAndWithout(t *testing.T) {
	assert.True(t, sample().Equal(sample()))
	a := sample().Without(ProxyFile)
	b := sample()
	b[ProxyFile] = File{Data: []byte("other")}
	assert.False(t, sample().Equal(b))
	assert.True(t, a.Equal(b.Without(ProxyFile)))
	assert.Len(t, sample(), 4, "Without does not change the original")
}

func TestValidate(t *testing.T) {
	assert.NoError(t, sample().Validate())
	assert.ErrorContains(t, sample().Without(ConfigFile, DockerfileFile).Validate(), "config.yml, container/Dockerfile")
}

// Escape sequences could hide lines of the approval diff in the terminal.
func TestValidateRefusesControlCharacters(t *testing.T) {
	for _, data := range []string{"a: b\n", "a: b\r\nc:\t\"ä ✓\"\r\n"} {
		s := sample()
		s[ConfigFile] = File{Data: []byte(data)}
		assert.NoError(t, s.Validate(), "%q", data)
	}
	for _, p := range []string{ConfigFile, ProxyFile} {
		s := sample()
		s[p] = File{Data: []byte("a: b\n\x1b[8mc: d\x1b[0m\n")}
		err := s.Validate()
		assert.ErrorContains(t, err, "line 2: control character 0x1b")
		assert.ErrorContains(t, err, p)
	}
	s := sample()
	s["container/logo.png"] = File{Data: []byte("\x89PNG\r\n\x1a\n\x00")}
	assert.NoError(t, s.Validate(), "container/ may hold binary files")
}

func TestWorktreeAndCommitHaveTheSameHash(t *testing.T) {
	dir := testgit.Init(t)
	writeSample(t, dir)
	testgit.WriteFile(t, dir, ".gitignore", ".DS_Store\n", 0o644)
	testgit.WriteFile(t, dir, ".crannog/.DS_Store", "junk", 0o644)
	testgit.WriteFile(t, dir, "outside.txt", "x", 0o644)
	testgit.Git(t, dir, "add", ".")
	testgit.Git(t, dir, "commit", "-q", "-m", "setup")

	wt, err := ReadWorktree(dir)
	require.NoError(t, err)
	commit, err := ReadCommit(dir, "HEAD")
	require.NoError(t, err)

	assert.Equal(t, sample().Hash(), wt.Hash(), "ignored files do not count")
	assert.Equal(t, wt.Hash(), commit.Hash())
	assert.True(t, commit["container/run.sh"].Executable)
}

func TestReadWorktreeUncommitted(t *testing.T) {
	dir := testgit.Init(t)
	writeSample(t, dir)
	testgit.Git(t, dir, "add", ".")
	testgit.Git(t, dir, "commit", "-q", "-m", "setup")

	changed, err := HasChanges(dir)
	require.NoError(t, err)
	assert.False(t, changed)

	require.NoError(t, testfs.Root(t, dir).Remove(filepath.Join(Dir, "container", "run.sh")))
	testgit.WriteFile(t, dir, ".crannog/new.txt", "new", 0o644)
	s, err := ReadWorktree(dir)
	require.NoError(t, err)
	assert.NotContains(t, s, "container/run.sh", "deleted files do not count")
	assert.Equal(t, "new", string(s["new.txt"].Data), "untracked files count")

	changed, err = HasChanges(dir)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestReadMissingSetup(t *testing.T) {
	dir := testgit.Init(t)
	s, err := ReadWorktree(dir)
	require.NoError(t, err)
	assert.Empty(t, s)
	s, err = ReadCommit(dir, "HEAD")
	require.NoError(t, err)
	assert.Empty(t, s)
}

func TestSymlinksAreAnError(t *testing.T) {
	dir := testgit.Init(t)
	writeSample(t, dir)
	wt := testfs.Root(t, dir)
	require.NoError(t, wt.Symlink("/etc/passwd", filepath.Join(Dir, "link")))
	_, err := ReadWorktree(dir)
	assert.ErrorContains(t, err, "symlink")

	testgit.Git(t, dir, "add", ".")
	testgit.Git(t, dir, "commit", "-q", "-m", "link")
	_, err = ReadCommit(dir, "HEAD")
	assert.ErrorContains(t, err, "not a regular file")

	_, err = ReadDir(wt, Dir)
	assert.ErrorContains(t, err, "symlink")
}

// .crannog/ deleted in the working tree, but still in the index, is
// missing: the setup is empty.
func TestReadWorktreeDeletedSetup(t *testing.T) {
	dir := testgit.Init(t)
	writeSample(t, dir)
	testgit.Git(t, dir, "add", ".")
	testgit.Git(t, dir, "commit", "-q", "-m", "setup")
	require.NoError(t, testfs.Root(t, dir).RemoveAll(Dir))

	s, err := ReadWorktree(dir)
	require.NoError(t, err)
	assert.Empty(t, s)
}

// A .crannog that is a file or a symlink is an error, not a missing setup.
func TestNonDirectorySetupIsAnError(t *testing.T) {
	for name, create := range map[string]func(t *testing.T, dir string){
		"file": func(t *testing.T, dir string) {
			testgit.WriteFile(t, dir, Dir, "x", 0o644)
		},
		"symlink": func(t *testing.T, dir string) {
			require.NoError(t, testfs.Root(t, dir).Symlink("elsewhere", Dir))
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := testgit.Init(t)
			create(t, dir)

			_, err := ReadWorktree(dir) // untracked
			assert.ErrorContains(t, err, "not a directory")

			testgit.Git(t, dir, "add", ".")
			_, err = ReadWorktree(dir) // tracked
			assert.ErrorContains(t, err, "not a directory")

			testgit.Git(t, dir, "commit", "-q", "-m", "setup")
			_, err = ReadCommit(dir, "HEAD")
			assert.ErrorContains(t, err, "not a directory")

			// Deleted in the working tree: nothing to read.
			require.NoError(t, testfs.Root(t, dir).Remove(Dir))
			s, err := ReadWorktree(dir)
			require.NoError(t, err)
			assert.Empty(t, s)
		})
	}
}

// A symlinked .crannog/ or directory in it never leads out of the
// worktree's .crannog/ (ADR 0032).
func TestReadWorktreeStaysInTheSetup(t *testing.T) {
	dir := testgit.Init(t)
	writeSample(t, dir)
	testgit.Git(t, dir, "add", ".")
	testgit.Git(t, dir, "commit", "-q", "-m", "setup")
	wt := testfs.Root(t, dir)
	testfs.WriteFile(t, wt, "elsewhere/Dockerfile", "secret", 0o644)

	// Tracked files below a directory that became an ignored symlink: git
	// lists the tracked files only.
	require.NoError(t, wt.RemoveAll(filepath.Join(Dir, "container")))
	require.NoError(t, wt.Symlink("../elsewhere", filepath.Join(Dir, "container")))
	testfs.WriteFile(t, wt, ".git/info/exclude", "/.crannog/container\n", 0o644)
	_, err := ReadWorktree(dir)
	assert.ErrorContains(t, err, "escapes")

	// .crannog/ itself replaced by a symlink.
	require.NoError(t, wt.Rename(Dir, "moved"))
	require.NoError(t, wt.Symlink("moved", Dir))
	_, err = ReadWorktree(dir)
	assert.ErrorContains(t, err, "not a directory")
}

func TestWriteAndReadDir(t *testing.T) {
	root, _ := testfs.TempRoot(t)
	require.NoError(t, sample().Write(root, "copy"))
	s, err := ReadDir(root, "copy")
	require.NoError(t, err)
	assert.Equal(t, sample().Hash(), s.Hash())
	info, err := root.Stat("copy/container/run.sh")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	assert.Error(t, Setup{"../x": {}}.Write(root, "bad"))
	assert.False(t, testfs.Exists(t, root, "x"))
}

func TestPresent(t *testing.T) {
	dir := testgit.Init(t)
	ok, err := Present(dir)
	require.NoError(t, err)
	assert.False(t, ok)
	writeSample(t, dir)
	ok, err = Present(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestReadFS(t *testing.T) {
	fsys := fstest.MapFS{
		"config.yml":           {Data: []byte("a")},
		"container/Dockerfile": {Data: []byte("b"), Mode: 0o755},
	}
	s, err := ReadFS(fsys)
	require.NoError(t, err)
	assert.Equal(t, Setup{
		"config.yml":           {Data: []byte("a")},
		"container/Dockerfile": {Data: []byte("b"), Executable: true},
	}, s)
}

func TestStore(t *testing.T) {
	root, _ := testfs.TempRoot(t)
	st := Store{Root: root, Dir: "repositories/1/approved"}

	latest, err := st.Latest()
	require.NoError(t, err)
	assert.Empty(t, latest)

	hash, err := st.Add(sample())
	require.NoError(t, err)
	assert.Equal(t, sample().Hash(), hash)
	again, err := st.Add(sample())
	require.NoError(t, err)
	assert.Equal(t, hash, again)

	ok, err := st.Has(hash)
	require.NoError(t, err)
	assert.True(t, ok)
	got, err := st.Get(hash)
	require.NoError(t, err)
	assert.True(t, sample().Equal(got))

	require.NoError(t, st.SetLatest(hash))
	latest, err = st.Latest()
	require.NoError(t, err)
	assert.Equal(t, hash, latest)

	entries, err := fs.ReadDir(root.FS(), st.Dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temporary dirs left")

	testfs.WriteFile(t, root, filepath.Join(st.Dir, hash, "config.yml"), "tampered", 0o644)
	_, err = st.Get(hash)
	assert.ErrorContains(t, err, "was changed")

	for _, bad := range []string{"../x", "", strings.Repeat("A", 64)} {
		_, err = st.Get(bad)
		assert.ErrorContains(t, err, "invalid setup hash", bad)
		_, err = st.Has(bad)
		assert.ErrorContains(t, err, "invalid setup hash", bad)
	}
}

// A symlink on the way to the store is refused (ADR 0032).
func TestStoreRefusesSymlinks(t *testing.T) {
	root, _ := testfs.TempRoot(t)
	require.NoError(t, root.Mkdir("elsewhere", 0o755))
	require.NoError(t, root.Mkdir("repositories", 0o755))
	require.NoError(t, root.Symlink("../elsewhere", "repositories/1"))
	st := Store{Root: root, Dir: "repositories/1/approved"}
	_, err := st.Add(sample())
	assert.ErrorContains(t, err, "not a directory")
	assert.ErrorContains(t, st.SetLatest(sample().Hash()), "not a directory")
	entries, err := fs.ReadDir(root.FS(), "elsewhere")
	require.NoError(t, err)
	assert.Empty(t, entries)
}
