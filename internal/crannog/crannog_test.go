package crannog

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/instructions"
	"github.com/hettiger/crannog/internal/preset"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/setup"
	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
	"github.com/hettiger/crannog/internal/ui"
)

func defaultSetup(t *testing.T) setup.Setup {
	h, err := home.Open(filepath.Join(t.TempDir(), ".crannog"))
	require.NoError(t, err)
	defer h.Close()
	p, err := preset.Find(h, preset.Default)
	require.NoError(t, err)
	s, err := p.Load()
	require.NoError(t, err)
	return s
}

func with(s setup.Setup, path, content string) setup.Setup {
	c := s.Without()
	c[path] = setup.File{Data: []byte(content)}
	return c
}

func TestPlanChange(t *testing.T) {
	s := defaultSetup(t)
	assert.Equal(t, changeCreate, planChange(nil, s))
	assert.Equal(t, changeNone, planChange(s, s.Without()))
	assert.Equal(t, changeReload, planChange(s, with(s, setup.ProxyFile, "x")))
	assert.Equal(t, changeRebuild, planChange(s, with(s, setup.ConfigFile, "x")))
	assert.Equal(t, changeRebuild, planChange(s, with(s, "container/extra", "x")))
	both := with(with(s, setup.ProxyFile, "x"), setup.ConfigFile, "y")
	assert.Equal(t, changeRebuild, planChange(s, both))
}

func TestDecide(t *testing.T) {
	tests := []struct {
		name       string
		c          change
		approved   bool
		fromPreset bool
		sessions   int
		checkout   bool
		want       question
	}{
		{"unchanged", changeNone, false, false, 0, true, askNothing},
		{"unchanged, second session", changeNone, true, false, 1, true, askSecondSession},
		{"unchanged, apply", changeNone, true, false, 1, false, askNothing},
		{"new content", changeCreate, false, false, 0, true, askApproval},
		{"approved before", changeCreate, true, false, 0, true, askNothing},
		{"preset", changeCreate, false, true, 0, true, askNothing},
		{"rebuild, new content, sessions", changeRebuild, false, false, 2, true, askApproval},
		{"rebuild, approved, sessions", changeRebuild, true, false, 2, false, askRebuild},
		{"rebuild, preset, sessions", changeRebuild, false, true, 1, true, askRebuild},
		{"rebuild, approved, no session", changeRebuild, true, false, 0, true, askNothing},
		{"reload, approved, sessions, checkout", changeReload, true, false, 1, true, askSecondSession},
		{"reload, approved, sessions, apply", changeReload, true, false, 1, false, askNothing},
		{"reload, new content", changeReload, false, false, 1, false, askApproval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decide(tt.c, tt.approved, tt.fromPreset, tt.sessions, tt.checkout))
		})
	}
}

func writeSetup(t *testing.T, dir string, s setup.Setup) {
	wt := testfs.Root(t, dir)
	require.NoError(t, wt.RemoveAll(setup.Dir))
	require.NoError(t, s.Write(wt, setup.Dir))
}

func TestReadSource(t *testing.T) {
	f := newFixture(t)
	testgit.Git(t, f.dir, "branch", "other")

	src, err := f.m.readSource("main")
	require.NoError(t, err)
	assert.True(t, src.preset)
	assert.Equal(t, "from preset default", src.desc)

	s := with(defaultSetup(t), "container/extra", "x")
	writeSetup(t, f.dir, s)
	src, err = f.m.readSource("main")
	require.NoError(t, err)
	assert.False(t, src.preset)
	assert.Equal(t, "from working tree (uncommitted changes)", src.desc)
	assert.Equal(t, s.Hash(), src.setup.Hash())

	src, err = f.m.readSource("other")
	require.NoError(t, err)
	assert.True(t, src.preset, "uncommitted changes of another worktree are not used")
	assert.Contains(t, f.out.String(), "uncommitted changes to .crannog/ in this worktree are not used")

	testgit.Git(t, f.dir, "add", ".")
	testgit.Git(t, f.dir, "commit", "-q", "-m", "setup")
	testgit.Git(t, f.dir, "switch", "-q", "other")
	src, err = f.m.readSource("main")
	require.NoError(t, err)
	assert.Regexp(t, `^from main \(commit [0-9a-f]+\)$`, src.desc)
	assert.Equal(t, s.Hash(), src.setup.Hash())

	writeSetup(t, f.dir, with(s, setup.ConfigFile, "unknown: 1"))
	_, err = f.m.readSource("other")
	assert.ErrorContains(t, err, "field unknown not found", "invalid config fails before any approval")
}

func TestReviewApproval(t *testing.T) {
	f := newFixture(t)
	tg := f.target("main")
	s := with(defaultSetup(t), "container/extra", "x")
	writeSetup(t, f.dir, s)
	src, err := f.m.readSource("main")
	require.NoError(t, err)

	_, _, err = f.m.review(tg, src, nil, 0, true)
	assert.ErrorContains(t, err, "needs your approval", "no terminal")

	f.m.UI.Yes = true
	_, _, err = f.m.review(tg, src, nil, 0, true)
	assert.ErrorContains(t, err, "--yes does not approve")

	f.prompt.terminal = true
	_, _, err = f.m.review(tg, src, nil, 0, true)
	assert.ErrorIs(t, err, ui.ErrDeclined)
	ok, _ := f.m.store().Has(s.Hash())
	assert.False(t, ok, "declined content is not stored")

	f.prompt.answer = true
	c, hash, err := f.m.review(tg, src, nil, 0, true)
	require.NoError(t, err)
	assert.Equal(t, changeCreate, c)
	assert.Equal(t, s.Hash(), hash)
	latest, _ := f.m.store().Latest()
	assert.Equal(t, hash, latest)
	assert.Contains(t, f.out.String(), "+++ .crannog/container/extra", "first build shows everything")

	// Approved once, approved always (ADR 0024).
	f.prompt.asked = nil
	_, _, err = f.m.review(f.target("other"), src, nil, 0, true)
	require.NoError(t, err)
	assert.Empty(t, f.prompt.asked)
}

func TestReviewListsHostVariables(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	cfg := strings.Replace(string(s[setup.ConfigFile].Data), "env:\n", "env:\n  GH_TOKEN: ${GH_TOKEN}\n", 1)
	writeSetup(t, f.dir, with(s, setup.ConfigFile, cfg))
	src, err := f.m.readSource("main")
	require.NoError(t, err)
	f.prompt.terminal, f.prompt.answer = true, true
	_, _, err = f.m.review(f.target("main"), src, nil, 0, true)
	require.NoError(t, err)
	assert.Contains(t, f.out.String(), "reads these variables from your environment: GH_TOKEN")
}

func TestReviewPresetNeedsNoApproval(t *testing.T) {
	f := newFixture(t)
	src, err := f.m.readSource("main")
	require.NoError(t, err)
	c, hash, err := f.m.review(f.target("main"), src, nil, 0, true)
	require.NoError(t, err)
	assert.Equal(t, changeCreate, c)
	assert.Empty(t, f.prompt.asked)
	ok, _ := f.m.store().Has(hash)
	assert.True(t, ok, "stored as an approved setup")
	latest, _ := f.m.store().Latest()
	assert.Equal(t, hash, latest)
}

// existing sets up a stopped crannog using s, as if checkout built it.
func (f *fixture) existing(branch string, s setup.Setup) *target {
	tg := f.target(branch)
	hash, err := f.m.store().Add(s)
	require.NoError(f.t, err)
	require.NoError(f.t, tg.dir.mkdirs(proxyDir, caDir))
	require.NoError(f.t, tg.dir.WriteBranch(branch))
	require.NoError(f.t, tg.dir.WriteHash(hash))
	require.NoError(f.t, f.m.writeProxyConfig(tg, s))
	f.rt.images[s.ImageName()] = true
	_, cfg, err := f.m.loadApproved(tg, hash)
	require.NoError(f.t, err)
	f.rt.images[cfg.Proxy.Image] = true
	require.NoError(f.t, f.m.createContainers(tg, s, cfg))
	f.rt.calls = nil
	return tg
}

func TestApplyWithoutCrannog(t *testing.T) {
	f := newFixture(t)
	assert.ErrorContains(t, f.m.Apply(""), "start one with crannog checkout main")
}

func TestApplyUpToDate(t *testing.T) {
	f := newFixture(t)
	f.existing("main", defaultSetup(t))
	require.NoError(t, f.m.Apply(""))
	assert.Contains(t, f.out.String(), "Using .crannog/ from preset default")
	assert.Contains(t, f.out.String(), "is up to date")
	assert.Empty(t, f.rt.calls)
}

func TestApplyReloadsTheProxy(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	tg := f.existing("main", s)
	f.rt.containers[tg.names.Proxy].State = runtime.StateRunning
	f.rt.containers[tg.names.Container].State = runtime.StateRunning

	// A running session.
	sess, _, err := tg.dir.registerSession()
	require.NoError(t, err)
	defer sess.Unlock()

	next := with(s, setup.ProxyFile, string(s[setup.ProxyFile].Data)+"# changed\n")
	writeSetup(t, f.dir, next)
	f.prompt.terminal, f.prompt.answer = true, true
	require.NoError(t, f.m.Apply(""))

	assert.Equal(t, []string{"exec " + tg.names.Proxy + " sh"}, f.rt.calls, "reload, no rebuild")
	data, err := readProxyFile(tg)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# changed")
	hash, _ := tg.dir.Hash()
	assert.Equal(t, next.Hash(), hash)
	assert.Len(t, f.prompt.asked, 1, "one approval, no confirmation")
}

func TestApplyRebuildsAndLeavesTheCrannogStopped(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	tg := f.existing("main", s)
	f.rt.containers[tg.names.Proxy].State = runtime.StateRunning
	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	sess, _, err := tg.dir.registerSession()
	require.NoError(t, err)
	defer sess.Unlock()

	next := with(s, "container/extra", "x")
	_, err = f.m.store().Add(next)
	require.NoError(t, err)
	writeSetup(t, f.dir, next)

	// Approved content with a running session: a confirmation.
	assert.ErrorContains(t, f.m.Apply(""), "pass --yes")
	f.m.UI.Yes = true
	require.NoError(t, f.m.Apply(""))
	assert.Contains(t, f.out.String(), "1 session running in this crannog end when it is rebuilt")

	assert.Equal(t, []string{
		"build " + next.ImageName(),
		"delete " + tg.names.Container + " " + tg.names.Proxy,
		"create " + tg.names.Proxy + " " + "docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb",
		"create " + tg.names.Container + " " + next.ImageName(),
	}, f.rt.calls, "the image is built before the old containers go")
	assert.Equal(t, runtime.StateStopped, f.rt.containers[tg.names.Container].State)
	hash, _ := tg.dir.Hash()
	assert.Equal(t, next.Hash(), hash)
}

func TestCheckoutWithMissingContainers(t *testing.T) {
	s := defaultSetup(t)
	for _, tc := range []struct {
		name  string
		setup setup.Setup
		hint  string
	}{
		{"unchanged", s, "the work volume may still contain work; crannog destroy deletes it"},
		{"rebuild", with(s, "container/extra", "x"), "crannog apply main recreates them with this setup and keeps the work volume"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			tg := f.existing("main", s)
			require.NoError(t, f.rt.Delete(tg.names.Container, tg.names.Proxy))
			err := f.m.ensureRunning(tg, &source{setup: tc.setup})
			require.ErrorContains(t, err, "the containers of the crannog of main are missing; "+tc.hint)
		})
	}
}

func TestSelfRepair(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	idle := f.existing("main", s)
	busy := f.existing("busy", s)
	for _, tg := range []*target{idle, busy} {
		f.rt.containers[tg.names.Proxy].State = runtime.StateRunning
		f.rt.containers[tg.names.Container].State = runtime.StateRunning
		require.NoError(t, f.m.addRemote(tg.names))
	}
	sess, _, err := busy.dir.registerSession()
	require.NoError(t, err)
	defer sess.Unlock()

	stopped := f.existing("stopped", s)
	require.NoError(t, f.m.addRemote(stopped.names))
	testgit.Git(t, f.dir, "remote", "add", "crannog-mine", "git@example.com:x")
	testgit.Git(t, f.dir, "remote", "add", "crannog-orphan-12345678", "crannog::orphan-12345678")

	f.m.SelfRepair()

	assert.Equal(t, runtime.StateStopped, f.rt.containers[idle.names.Container].State)
	assert.Equal(t, runtime.StateStopped, f.rt.containers[idle.names.Proxy].State)
	assert.Equal(t, runtime.StateRunning, f.rt.containers[busy.names.Container].State)
	remotes := testgit.Git(t, f.dir, "remote")
	assert.ElementsMatch(t, []string{"crannog-mine", busy.names.Remote}, strings.Fields(remotes))
}

func TestSelfRepairSkipsLockedCrannogs(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	lock, err := tg.dir.tryLock()
	require.NoError(t, err)
	defer lock.Unlock()
	f.m.SelfRepair()
	assert.Equal(t, runtime.StateRunning, f.rt.containers[tg.names.Container].State)
}

func TestList(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	f.existing("main", s)
	b := f.existing("feature/x", s)
	f.rt.containers[b.names.Container].State = runtime.StateRunning
	sess, _, err := b.dir.registerSession()
	require.NoError(t, err)
	defer sess.Unlock()
	gone := f.existing("gone", s)
	delete(f.rt.containers, gone.names.Container)
	half := f.target("half")
	require.NoError(t, half.dir.WriteBranch("half"))

	list, err := f.m.List()
	require.NoError(t, err)
	assert.Equal(t, []Info{
		{Branch: "feature/x", Status: runtime.StateRunning, Sessions: 1},
		{Branch: "gone", Status: StatusMissing},
		{Branch: "half", Status: StatusIncomplete},
		{Branch: "main", Status: runtime.StateStopped},
	}, list)
}

func TestTargetSafetyNet(t *testing.T) {
	f := newFixture(t)
	tg := f.target("main")
	require.NoError(t, tg.dir.WriteBranch("something-else"))
	_, err := f.m.target("main")
	assert.ErrorContains(t, err, `belongs to branch "something-else"`)
}

func TestRemotes(t *testing.T) {
	f := newFixture(t)
	n := f.target("main").names
	require.NoError(t, f.m.addRemote(n))
	require.NoError(t, f.m.addRemote(n), "idempotent")
	assert.Equal(t, "never", testgit.Git(t, f.dir, "config", "remote."+n.Remote+".followRemoteHEAD"))
	require.NoError(t, f.m.removeRemote(n))
	assert.Empty(t, testgit.Git(t, f.dir, "remote"))

	testgit.Git(t, f.dir, "remote", "add", n.Remote, "https://example.com/x")
	assert.ErrorContains(t, f.m.addRemote(n), "already exists")
	require.NoError(t, f.m.removeRemote(n))
	assert.Equal(t, n.Remote, testgit.Git(t, f.dir, "remote"), "the user's remote stays")
}

func TestExitCode(t *testing.T) {
	code, err := exitCode(exec.Command("sh", "-c", "exit 42").Run())
	require.NoError(t, err)
	assert.Equal(t, 42, code)
	code, err = exitCode(exec.Command("sh", "-c", "kill -TERM $$").Run())
	require.NoError(t, err)
	assert.Equal(t, 143, code)
	code, err = exitCode(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	_, err = exitCode(exec.Command("/nonexistent").Run())
	assert.Error(t, err)
}

// The managed instructions block is written in the crannog, not on the host.
func TestWriteInstructions(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	claude := "/home/agent/.crannog/agents/claude/CLAUDE.md"
	f.rt.files[claude] = "# mine\n"
	require.NoError(t, hostfs.MkdirAll(f.m.Home.Root, home.Agents))

	require.NoError(t, f.m.writeInstructions(tg))
	assert.Equal(t, "# mine\n\n"+instructions.ManagedBlock+"\n", f.rt.files[claude])
	for _, a := range instructions.Agents {
		assert.Contains(t, f.rt.files["/home/agent/.crannog/agents/"+a.Name+"/"+a.File], instructions.ManagedBlock)
	}
	entries, err := fs.ReadDir(f.m.Home.Root.FS(), home.Agents)
	require.NoError(t, err)
	assert.Empty(t, entries, "the host does not write into the agent homes")

	f.rt.calls = nil
	require.NoError(t, f.m.writeInstructions(tg))
	assert.Len(t, f.rt.calls, len(instructions.Agents), "up to date: reads only")
}
