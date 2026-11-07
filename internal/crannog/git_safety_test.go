package crannog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
	"github.com/hettiger/crannog/internal/ui"
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

func TestLossesIsolatesRefsTagsAndMaintenance(t *testing.T) {
	f := newFixture(t)
	source := testgit.Init(t)
	testgit.Git(t, source, "commit", "-q", "--allow-empty", "-m", "remote")
	testgit.Git(t, source, "tag", "remote-tag")
	tg := f.target("main")
	tg.names.URL = source
	f.rt.containers[tg.names.Container] = &runtime.Container{Name: tg.names.Container, State: runtime.StateRunning}
	f.rt.containers[tg.names.Proxy] = &runtime.Container{Name: tg.names.Proxy, State: runtime.StateRunning}
	head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
	old := "refs/crannog/destroy/" + tg.names.Slug + "/old"
	testgit.Git(t, f.dir, "update-ref", old, head)
	testgit.Git(t, f.dir, "tag", "host-only")
	testgit.Git(t, f.dir, "config", "maintenance.auto", "true")
	testgit.Git(t, f.dir, "config", "fetch.prune", "true")
	testgit.Git(t, f.dir, "config", "fetch.pruneTags", "true")
	traceDir := t.TempDir()
	t.Setenv("GIT_TRACE2_EVENT", filepath.Join(traceDir, "trace"))
	for range 2 {
		_, err := f.m.losses(tg)
		require.NoError(t, err)
	}
	assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", old))
	assert.Equal(t, old, testgit.Git(t, f.dir, "for-each-ref", "--format=%(refname)", "refs/crannog/destroy/"))
	assert.Empty(t, testgit.Git(t, f.dir, "tag", "--list", "remote-tag"))
	assert.Equal(t, "host-only", testgit.Git(t, f.dir, "tag", "--list", "host-only"))
	trace := testfs.ReadFile(t, testfs.Root(t, traceDir), "trace")
	assert.NotContains(t, trace, `"argv":["git","maintenance"`)
}

func TestDeleteRefsDoesNotDereferenceSymbolicRefs(t *testing.T) {
	f := newFixture(t)
	head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
	testgit.Git(t, f.dir, "update-ref", "refs/heads/precious", head)
	testgit.Git(t, f.dir, "symbolic-ref", "refs/crannog/destroy/test/link", "refs/heads/precious")
	f.m.deleteRefs("refs/crannog/destroy/test/")
	assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", "refs/heads/precious"))
	assert.Empty(t, testgit.Git(t, f.dir, "for-each-ref", "--format=%(refname)", "refs/crannog/destroy/test/"))
}

func TestLossesMissingContainersPreservesExistingRefs(t *testing.T) {
	f := newFixture(t)
	tg := f.target("main")
	head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
	ref := "refs/crannog/destroy/" + tg.names.Slug + "/old"
	testgit.Git(t, f.dir, "update-ref", ref, head)
	_, err := f.m.losses(tg)
	assert.ErrorContains(t, err, "containers are missing")
	assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", ref))
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

func TestDestroyPreservesExistingRefs(t *testing.T) {
	for _, missing := range []bool{false, true} {
		name := "declined after loss check"
		if missing {
			name = "declined with missing containers"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			tg := f.target("main")
			require.NoError(t, tg.dir.WriteBranch("main"))
			helperDir := t.TempDir()
			testfs.WriteFile(t, testfs.Root(t, helperDir), "git-remote-crannog", "#!/bin/sh\nexit 1\n", 0o755)
			t.Setenv("PATH", helperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if !missing {
				source := testgit.Init(t)
				testgit.Git(t, f.dir, "config", "url."+source+".insteadOf", tg.names.URL)
				f.rt.containers[tg.names.Container] = &runtime.Container{Name: tg.names.Container, State: runtime.StateRunning}
				f.rt.containers[tg.names.Proxy] = &runtime.Container{Name: tg.names.Proxy, State: runtime.StateRunning}
				sess, _, err := tg.dir.registerSession()
				require.NoError(t, err)
				defer sess.Unlock()
			}
			head := testgit.Git(t, f.dir, "rev-parse", "HEAD")
			ref := "refs/crannog/destroy/" + tg.names.Slug + "/old"
			testgit.Git(t, f.dir, "update-ref", ref, head)
			f.prompt.terminal = true
			err := f.m.Destroy("main")
			assert.ErrorIs(t, err, ui.ErrDeclined)
			if missing {
				assert.Contains(t, f.out.String(), "containers are missing")
			} else {
				assert.NotContains(t, f.out.String(), "cannot check what would be lost")
			}
			assert.Equal(t, head, testgit.Git(t, f.dir, "rev-parse", ref))
		})
	}
}
