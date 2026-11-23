package git

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testgit"
)

const (
	remote = "crannog-main-0123abcd"
	url    = "crannog::main-0123abcd"
)

func TestReading(t *testing.T) {
	dir := testgit.Init(t)

	top, err := TopLevel(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, top)

	b, err := CurrentBranch(dir)
	require.NoError(t, err)
	assert.Equal(t, "main", b)
	testgit.Git(t, dir, "checkout", "-q", "--detach")
	b, err = CurrentBranch(dir)
	require.NoError(t, err)
	assert.Empty(t, b, "detached")

	ok, err := HasBranch(dir, "main")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = HasBranch(dir, "main^")
	require.NoError(t, err)
	assert.False(t, ok, "not a revision expression")

	commit, err := BranchCommit(dir, "main")
	require.NoError(t, err)
	assert.Equal(t, testgit.Git(t, dir, "rev-parse", "--short", "main"), commit)

	_, ok, err = Config(dir, "user.name")
	require.NoError(t, err)
	assert.False(t, ok)
	testgit.Git(t, dir, "config", "user.name", "Ada")
	v, ok, err := Config(dir, "user.name")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Ada", v)
}

func TestRemotes(t *testing.T) {
	dir := testgit.Init(t)
	urls, err := RemoteURLs(dir)
	require.NoError(t, err)
	assert.Empty(t, urls)
	current, err := RemoteURL(dir, remote)
	require.NoError(t, err)
	assert.Empty(t, current)

	require.NoError(t, AddRemote(dir, remote, url))
	testgit.Git(t, dir, "remote", "add", "crannog-mine", "https://example.com/x")
	testgit.Git(t, dir, "remote", "add", "origin", "https://example.com/origin")
	assert.Equal(t, "never", testgit.Git(t, dir, "config", "remote."+remote+".followRemoteHEAD"))
	urls, err = RemoteURLs(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{remote: url, "crannog-mine": "https://example.com/x"}, urls)

	require.NoError(t, RemoveRemote(dir, remote, url))
	require.NoError(t, RemoveRemote(dir, remote, url), "no remote, nothing to do")
	assert.Equal(t, "crannog-mine\norigin", testgit.Git(t, dir, "remote"))
}

// A remote with the crannog's name but another URL is the user's.
func TestRemoveRemoteLeavesTheUsersRemote(t *testing.T) {
	dir := testgit.Init(t)
	testgit.Git(t, dir, "remote", "add", remote, "https://example.com/x")
	require.NoError(t, RemoveRemote(dir, remote, url))
	assert.Equal(t, remote, testgit.Git(t, dir, "remote"))
}

// The writing functions accept only a crannog's remote and URL (ADR 0031).
func TestWritingRefusesWhatIsNotACrannogs(t *testing.T) {
	dir := testgit.Init(t)
	testgit.Git(t, dir, "remote", "add", "origin", "https://example.com/origin")
	config := testgit.Git(t, dir, "config", "--local", "--list")

	for _, tc := range []struct{ name, url string }{
		{"origin", url},
		{"crannog-", url},
		{"crannog-mine", url},
		{"--mirror", url},
		{remote, "https://example.com/x"},
		{remote, "origin"},
		{remote, "crannog::../x"},
		{remote, "crannog::other-0123abcd"},
	} {
		assert.Error(t, AddRemote(dir, tc.name, tc.url), "add %s %s", tc.name, tc.url)
		assert.Error(t, RemoveRemote(dir, tc.name, tc.url), "remove %s %s", tc.name, tc.url)
	}
	_, err := RemoteURL(dir, "origin")
	assert.Error(t, err)

	var out bytes.Buffer
	for _, u := range []string{"origin", "https://example.com/x", dir, "--all", "crannog::"} {
		assert.Error(t, PushToCrannog(dir, u, "main", &out), "push to %s", u)
	}
	assert.Error(t, PushToCrannog(dir, url, "", &out))
	assert.Empty(t, out.String(), "git did not run")
	assert.Equal(t, config, testgit.Git(t, dir, "config", "--local", "--list"))
}

// A value that is an argument of its own is never an option.
func TestArgumentsAreNotOptions(t *testing.T) {
	dir := testgit.Init(t)
	_, err := Tree(dir, "--output=x", ".")
	assert.ErrorContains(t, err, "invalid revision")
	_, err = Blob(dir, "--batch")
	assert.ErrorContains(t, err, "invalid object id")
	_, err = Blob(dir, "")
	assert.ErrorContains(t, err, "invalid object id")
	_, _, err = Config(dir, "--unset-all")
	assert.ErrorContains(t, err, "invalid configuration key")
}

func TestDiffDirs(t *testing.T) {
	dir := testgit.Init(t)
	testgit.WriteFile(t, dir, "a/f", "one\n", 0o644)
	testgit.WriteFile(t, dir, "b/f", "two\n", 0o644)
	var out bytes.Buffer
	require.NoError(t, DiffDirs(dir, "a", "b", &out, false))
	assert.Contains(t, out.String(), "-one\n+two\n")

	out.Reset()
	require.NoError(t, DiffDirs(dir, "a", "a", &out, false))
	assert.Empty(t, out.String())
}
