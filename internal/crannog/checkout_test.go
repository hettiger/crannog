package crannog

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/setup"
	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
	"github.com/hettiger/crannog/internal/ui"
)

// The whole life of a crannog on the fake runtime: the first checkout
// builds and runs a session, the second starts the stopped crannog, destroy
// removes everything. The session command is run by the fake, the initial
// transfer and the destroy check go to a bare repository.
func TestCheckoutBuildsRunsAndStops(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.target("main")
	bare := f.redirectURL(tg)
	testgit.Git(t, f.dir, "tag", "v1")
	testgit.Git(t, f.dir, "branch", "other")

	code, err := f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	assert.Equal(t, 0, code)
	assert.Empty(t, f.rt.callsWith("delete"), "a fresh crannog has nothing to remove")
	assert.Empty(t, f.rt.callsWith("network delete"))
	assert.Empty(t, f.rt.callsWith("volume delete"))
	s := defaultSetup(t)
	assert.Equal(t, []string{
		"build " + s.ImageName(),
		"pull docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb",
		"network create " + tg.names.Network,
		"volume create " + tg.names.WorkVolume,
		"volume create " + tg.names.StateVolume,
		"create " + tg.names.Proxy + " docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb",
		"create " + tg.names.Container + " " + s.ImageName(),
		"start " + tg.names.Proxy,
		"start " + tg.names.Container,
		"stop " + tg.names.Container + " " + tg.names.Proxy,
	}, withoutExec(f.rt.calls), "built, started, stopped after the session")
	out := f.out.String()
	assert.Contains(t, out, "Using .crannog/ from preset default")
	assert.Contains(t, out, "Transferring the repository")
	assert.Contains(t, out, "Stopping the crannog of main")

	hash, err := tg.dir.Hash()
	require.NoError(t, err)
	assert.Equal(t, s.Hash(), hash, "the crannog uses the preset")
	branch, err := tg.dir.Branch()
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
	assert.Equal(t, f.m.Repo.CommonDir+"\n", testfs.ReadFile(t, f.m.CrannogHome.Root, filepath.Join(home.RepositoryPath(f.m.Repo.ID), home.PathFile)))
	assert.Equal(t, runtime.StateStopped, f.rt.containers[tg.names.Container].State)
	assert.Equal(t, runtime.StateStopped, f.rt.containers[tg.names.Proxy].State)
	assert.Empty(t, testgit.Git(t, f.dir, "remote"), "the remote exists only while the crannog runs")
	n, err := tg.dir.sessions(false)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	refs := testgit.Git(t, bare, "for-each-ref", "--format=%(refname)")
	assert.Equal(t, []string{"refs/heads/main", "refs/remotes/origin/main", "refs/remotes/origin/other", "refs/tags/v1"},
		strings.Fields(refs), "all branches as origin/*, the tags and the start branch")

	// Second session: a warm start, nothing is built or created.
	f.rt.calls = nil
	f.out.Reset()
	code, err = f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	assert.Equal(t, 0, code)
	assert.Equal(t, []string{
		"start " + tg.names.Proxy,
		"start " + tg.names.Container,
		"stop " + tg.names.Container + " " + tg.names.Proxy,
	}, withoutExec(f.rt.calls))
	assert.NotContains(t, f.out.String(), "Transferring")

	// Destroy: the check fetches from the bare repository, then everything
	// goes except the image.
	f.rt.calls = nil
	f.m.UI.Yes = true
	require.NoError(t, f.m.Destroy(""), f.out.String())
	assert.Contains(t, f.out.String(), "all branch commits of the crannog of main were in your repository")
	assert.Equal(t, []string{
		"start " + tg.names.Proxy,
		"start " + tg.names.Container,
		"delete " + tg.names.Container + " " + tg.names.Proxy,
		"network delete " + tg.names.Network,
		"volume delete " + tg.names.WorkVolume,
		"volume delete " + tg.names.StateVolume,
	}, withoutExec(f.rt.calls))
	assert.True(t, f.rt.images[s.ImageName()], "the image stays")
	exists, err := tg.dir.exists()
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, testgit.Git(t, f.dir, "remote"))
	assert.Empty(t, testgit.Git(t, f.dir, "for-each-ref", "refs/crannog/"), "no temporary refs left")
}

// A failed build removes everything it created; the host directory goes
// too, so the next checkout starts from scratch.
func TestCheckoutRemovesAFailedBuild(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.target("main")
	// No redirect: the initial transfer fails through the stub helper.

	_, err := f.m.Checkout("", []string{"true"})
	require.ErrorContains(t, err, "initial transfer of the repository")
	assert.Contains(t, f.out.String(), "Building the crannog failed, removing what was created.")
	var deletes []string
	for _, c := range f.rt.calls {
		if strings.Contains(c, "delete") {
			deletes = append(deletes, c)
		}
	}
	assert.Equal(t, []string{
		"delete " + tg.names.Container + " " + tg.names.Proxy,
		"network delete " + tg.names.Network,
		"volume delete " + tg.names.WorkVolume,
		"volume delete " + tg.names.StateVolume,
	}, deletes)
	assert.Nil(t, f.rt.containers[tg.names.Container])
	assert.Nil(t, f.rt.containers[tg.names.Proxy])
	exists, err := tg.dir.exists()
	require.NoError(t, err)
	assert.False(t, exists, "host directory removed")
	assert.Empty(t, testgit.Git(t, f.dir, "remote"))
	assert.True(t, f.rt.images[defaultSetup(t).ImageName()], "the image stays")
	ok, err := f.m.store().Has(defaultSetup(t).Hash())
	require.NoError(t, err)
	assert.True(t, ok, "the approved setup stays")

	list, err := f.m.List()
	require.NoError(t, err)
	assert.Empty(t, list, "a directory without a branch is no crannog")
}

// Containers, a network or volumes with the names of a crannog the host
// does not know, e.g. after its host directory was deleted, are for the
// user to remove: checkout refuses, touches none of them and leaves no host
// directory behind.
func TestCheckoutRefusesExistingResources(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.target("main")
	f.rt.containers[tg.names.Container] = &runtime.Container{Name: tg.names.Container, State: runtime.StateRunning}
	f.rt.networks[tg.names.Network] = true
	f.rt.volumes[tg.names.WorkVolume] = true

	_, err := f.m.Checkout("", []string{"true"})
	require.EqualError(t, err, "cannot check out main: the following resources already exist\n"+
		"  container "+tg.names.Container+"\n"+
		"  network "+tg.names.Network+"\n"+
		"  volume "+tg.names.WorkVolume+"\n"+
		"Remove them, then run crannog checkout again.")
	assert.Empty(t, f.rt.calls, "nothing built, stopped or deleted")
	assert.Empty(t, f.prompt.asked, "refused before the approval")
	exists, err := tg.dir.exists()
	require.NoError(t, err)
	assert.False(t, exists)
	assert.ErrorContains(t, f.m.Destroy("main"), "there is no crannog for main")
}

// A declined first approval leaves no host directory behind, so destroy
// knows there is no crannog.
func TestDeclinedFirstCheckoutLeavesNoHostDirectory(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.target("main")
	writeSetup(t, f.dir, with(defaultSetup(t), "container/extra", "x"))
	f.prompt.terminal = true

	_, err := f.m.Checkout("", []string{"true"})
	require.ErrorIs(t, err, ui.ErrDeclined)
	assert.Empty(t, f.rt.calls)
	exists, err := tg.dir.exists()
	require.NoError(t, err)
	assert.False(t, exists)
	assert.ErrorContains(t, f.m.Destroy("main"), "there is no crannog for main")
}

// Remains of a build that was interrupted before it recorded its hash are
// removed before the crannog is built anew (ADR 0021).
func TestCheckoutRemovesAnInterruptedBuild(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.existing("main", defaultSetup(t))
	f.redirectURL(tg)
	require.NoError(t, f.m.CrannogHome.Root.Remove(filepath.Join(tg.dir.path, hashFile)))
	list, err := f.m.List()
	require.NoError(t, err)
	assert.Equal(t, []Info{{Branch: "main", Status: StatusIncomplete}}, list)

	f.rt.calls = nil
	_, err = f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	calls := withoutExec(f.rt.calls)
	assert.Equal(t, []string{
		"delete " + tg.names.Container + " " + tg.names.Proxy,
		"network delete " + tg.names.Network,
		"volume delete " + tg.names.WorkVolume,
		"volume delete " + tg.names.StateVolume,
	}, calls[:4], "the remains go first")
	assert.Equal(t, "network create "+tg.names.Network, calls[4])
	hash, err := tg.dir.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// A build that fails before the network and the volumes exist deletes
// neither: the runtime fails on deleting a missing one. The next checkout
// builds from scratch.
func TestCheckoutAfterABuildThatFailedEarly(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.target("main")
	f.redirectURL(tg)
	f.rt.buildErr = errors.New("build failed")

	_, err := f.m.Checkout("", []string{"true"})
	require.ErrorContains(t, err, "build failed")
	assert.NotContains(t, f.out.String(), "Warning")
	assert.Equal(t, []string{"build " + defaultSetup(t).ImageName()}, f.rt.calls)
	exists, err := tg.dir.exists()
	require.NoError(t, err)
	assert.False(t, exists, "host directory removed")

	f.rt.buildErr = nil
	_, err = f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	hash, err := tg.dir.Hash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// An interrupted build leaves any part of the runtime objects. Checkout and
// destroy delete the ones that exist and get past the missing ones.
func TestRemainsOfAnInterruptedBuildWithMissingObjects(t *testing.T) {
	interrupted := func(t *testing.T) (*fixture, *target) {
		f := newFixture(t)
		stubHelper(t)
		tg := f.target("main")
		f.redirectURL(tg)
		require.NoError(t, tg.dir.mkdirs(proxyDir, caDir))
		require.NoError(t, tg.dir.WriteBranch("main"))
		f.rt.networks[tg.names.Network] = true
		f.rt.volumes[tg.names.WorkVolume] = true
		return f, tg
	}
	deleted := func(tg *target) []string {
		return []string{"network delete " + tg.names.Network, "volume delete " + tg.names.WorkVolume}
	}

	t.Run("checkout", func(t *testing.T) {
		f, tg := interrupted(t)
		_, err := f.m.Checkout("", []string{"true"})
		require.NoError(t, err, f.out.String())
		assert.Equal(t, deleted(tg), f.rt.calls[:2], "the remains go first")
		hash, err := tg.dir.Hash()
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("destroy", func(t *testing.T) {
		f, tg := interrupted(t)
		f.prompt.answer = true
		f.prompt.terminal = true
		require.NoError(t, f.m.Destroy("main"), f.out.String())
		assert.Equal(t, deleted(tg), f.rt.calls)
		assert.Empty(t, f.rt.networks)
		assert.Empty(t, f.rt.volumes)
		exists, err := tg.dir.exists()
		require.NoError(t, err)
		assert.False(t, exists, "host directory removed")
	})
}

// If the proxy rejects a new iron-proxy.yaml, the old one is restored and
// the crannog keeps its setup.
func TestReloadRestoresTheOldProxyConfig(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	tg := f.existing("main", s)
	f.rt.containers[tg.names.Proxy].State = runtime.StateRunning
	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	sess, _, err := tg.dir.registerSession()
	require.NoError(t, err)
	defer sess.Unlock()
	reloads := 0
	f.rt.execHook = func(name string, args []string) error {
		if name == tg.names.Proxy && strings.Contains(args[2], proxyReloadURL) {
			reloads++
			if reloads == 1 {
				return assert.AnError
			}
		}
		return nil
	}

	next := with(s, setup.ProxyFile, "broken: [\n")
	writeSetup(t, f.dir, next)
	f.prompt.terminal, f.prompt.answer = true, true
	err = f.m.Apply("")
	require.ErrorContains(t, err, "the proxy rejected the new iron-proxy.yaml, the old one stays")
	assert.Equal(t, 2, reloads, "reloaded again with the old file")
	data, err := readProxyFile(tg)
	require.NoError(t, err)
	assert.Equal(t, s[setup.ProxyFile].Data, data)
	hash, err := tg.dir.Hash()
	require.NoError(t, err)
	assert.Equal(t, s.Hash(), hash)
	assert.Equal(t, runtime.StateRunning, f.rt.containers[tg.names.Container].State, "the session continues")
}

// withoutExec drops the exec calls (probes, scripts) from recorded calls.
func withoutExec(calls []string) []string {
	var out []string
	for _, c := range calls {
		if !strings.HasPrefix(c, "exec ") {
			out = append(out, c)
		}
	}
	return out
}

// A session that ends after destroy removed its crannog has nothing to
// stop and leaves no host directory behind.
func TestEndSessionOfADestroyedCrannog(t *testing.T) {
	f := newFixture(t)
	tg := f.target("main")

	require.NoError(t, f.m.endSession(tg))
	assert.Equal(t, "The crannog of main was destroyed while this session ran\n", f.out.String())
	assert.Empty(t, f.rt.calls)
	exists, err := tg.dir.exists()
	require.NoError(t, err)
	assert.False(t, exists, "the host directory is not created again")
}
