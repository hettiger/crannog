package setup

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// The approval diff must show the configuration as crannog reads it.
func TestValidateRefusesUnreviewableConfiguration(t *testing.T) {
	ok := []string{
		"a: b\n",
		"a: b\r\nc: d\r\n",
		"a:\t\"ä ✓ 🦫\"\n",
	}
	for _, data := range ok {
		s := sample()
		s[ConfigFile] = File{Data: []byte(data)}
		assert.NoError(t, s.Validate(), "%q", data)
	}

	bad := map[string]string{
		// UTF-16LE with BOM: YAML parsers accept it, git diff shows binary.
		"\xff\xfea\x00:\x00 \x00b\x00\n\x00": "not valid UTF-8",
		"\xef\xbb\xbfa: b\n":                 "U+FEFF",
		"a: b\x00\n":                         "U+0000",
		"a: b\n\x1b[8mc: d\x1b[0m\n":         "line 2: control or format character U+001B",
		"a: evil\rb: good\n":                 "U+000D",
		"a: \u202eb\n":                       "U+202E",
		"a: \u0085b\n":                       "U+0085",
	}
	for data, want := range bad {
		for _, p := range []string{ConfigFile, ProxyFile} {
			s := sample()
			s[p] = File{Data: []byte(data)}
			err := s.Validate()
			assert.ErrorContains(t, err, want, "%s %q", p, data)
			assert.ErrorContains(t, err, p, "%q", data)
		}
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

	require.NoError(t, os.Remove(filepath.Join(dir, Dir, "container/run.sh")))
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
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(dir, Dir, "link")))
	_, err := ReadWorktree(dir)
	assert.ErrorContains(t, err, "symlink")

	testgit.Git(t, dir, "add", ".")
	testgit.Git(t, dir, "commit", "-q", "-m", "link")
	_, err = ReadCommit(dir, "HEAD")
	assert.ErrorContains(t, err, "not a regular file")

	_, err = ReadDir(filepath.Join(dir, Dir))
	assert.ErrorContains(t, err, "symlink")
}

func TestWriteAndReadDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, sample().Write(dir))
	s, err := ReadDir(dir)
	require.NoError(t, err)
	assert.Equal(t, sample().Hash(), s.Hash())

	assert.Error(t, Setup{"../x": {}}.Write(filepath.Join(t.TempDir(), "bad")))
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
	st := Store{Dir: filepath.Join(t.TempDir(), "approved")}

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

	entries, err := os.ReadDir(st.Dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temporary dirs left")

	require.NoError(t, os.WriteFile(filepath.Join(st.Path(hash), "config.yml"), []byte("tampered"), 0o644))
	_, err = st.Get(hash)
	assert.ErrorContains(t, err, "was changed")
}
