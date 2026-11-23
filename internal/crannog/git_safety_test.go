package crannog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testgit"
)

func TestRemoveRemoteFetchMappings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mappings []string
		allowed  bool
	}{
		{name: "generated", mappings: []string{"generated"}, allowed: true},
		{name: "missing"},
		{name: "foreign", mappings: []string{"+refs/heads/*:refs/remotes/origin/*"}},
		{name: "multiple", mappings: []string{"generated", "+refs/heads/*:refs/remotes/origin/*"}},
		{name: "duplicate", mappings: []string{"generated", "generated"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			n := f.target("main").names
			testgit.Git(t, f.dir, "remote", "add", n.Remote, n.URL)
			key := "remote." + n.Remote + ".fetch"
			testgit.Git(t, f.dir, "config", "--unset-all", key)
			for _, mapping := range tc.mappings {
				if mapping == "generated" {
					mapping = "+refs/heads/*:refs/remotes/" + n.Remote + "/*"
				}
				testgit.Git(t, f.dir, "config", "--add", key, mapping)
			}
			head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
			testgit.Git(t, f.dir, "update-ref", "refs/remotes/origin/precious", head)
			err := f.m.removeRemote(n)
			if tc.allowed {
				require.NoError(t, err)
				assert.Empty(t, testgit.Git(t, f.dir, "remote"))
			} else {
				assert.ErrorContains(t, err, "unexpected fetch mappings")
				assert.Equal(t, n.Remote, testgit.Git(t, f.dir, "remote"))
			}
			assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", "refs/remotes/origin/precious"))
		})
	}
}

func TestInitialTransferIgnoresPushURL(t *testing.T) {
	f := newFixture(t)
	parent := t.TempDir()
	destination := filepath.Join(parent, "destination.git")
	foreign := filepath.Join(parent, "foreign.git")
	testgit.Git(t, "", "init", "--bare", "-q", destination)
	testgit.Git(t, "", "init", "--bare", "-q", foreign)
	tg := f.target("main")
	tg.names.URL = destination
	testgit.Git(t, f.dir, "remote", "add", tg.names.Remote, destination)
	testgit.Git(t, f.dir, "config", "remote."+tg.names.Remote+".pushurl", foreign)
	require.NoError(t, f.m.transferRepository(tg))
	head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
	assert.Equal(t, head, testgit.Git(t, destination, "rev-parse", "refs/heads/main"))
	assert.Empty(t, testgit.Git(t, foreign, "for-each-ref", "--format=%(refname)"))
}

func TestListRepairRefusesUnexpectedFetchMapping(t *testing.T) {
	f := newFixture(t)
	n := f.target("main").names
	testgit.Git(t, f.dir, "remote", "add", n.Remote, n.URL)
	testgit.Git(t, f.dir, "config", "remote."+n.Remote+".fetch", "+refs/heads/*:refs/remotes/origin/*")
	head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
	testgit.Git(t, f.dir, "update-ref", "refs/remotes/origin/precious", head)
	_, err := f.m.List()
	require.NoError(t, err)
	assert.Contains(t, f.out.String(), "unexpected fetch mappings")
	assert.Equal(t, n.Remote, testgit.Git(t, f.dir, "remote"))
	assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", "refs/remotes/origin/precious"))
}

func TestRemoveRemotePreservesSymbolicRefTarget(t *testing.T) {
	f := newFixture(t)
	n := f.target("main").names
	testgit.Git(t, f.dir, "remote", "add", n.Remote, n.URL)
	head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
	testgit.Git(t, f.dir, "update-ref", "refs/heads/precious", head)
	testgit.Git(t, f.dir, "symbolic-ref", "refs/remotes/"+n.Remote+"/link", "refs/heads/precious")
	require.NoError(t, f.m.removeRemote(n))
	assert.Empty(t, testgit.Git(t, f.dir, "remote"))
	assert.Empty(t, testgit.Git(t, f.dir, "for-each-ref", "--format=%(refname)", "refs/remotes/"+n.Remote+"/"))
	assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", "refs/heads/precious"))
}
