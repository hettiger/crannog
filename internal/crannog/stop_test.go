package crannog

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
)

// sessionFiles returns the contents of the session files of a crannog.
func sessionFiles(t *testing.T, tg *target) []string {
	t.Helper()
	r, err := tg.dir.open(sessionsDir, false)
	require.NoError(t, err)
	defer r.Close()
	entries, err := fs.ReadDir(r.FS(), ".")
	require.NoError(t, err)
	var files []string
	for _, e := range entries {
		files = append(files, testfs.ReadFile(t, r, e.Name()))
	}
	return files
}

// A detached checkout starts the crannog and returns while its session
// runs: the session only waits, counts like any other and keeps the
// crannog running until stop ends it.
func TestDetachedCheckoutAndStop(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.existing("main", defaultSetup(t))

	require.NoError(t, f.m.CheckoutDetached("", nil), f.out.String())
	assert.Equal(t, []string{
		"start " + tg.names.Proxy,
		"start " + tg.names.Container,
		"detach " + tg.names.Container + " " + waitCommand,
	}, withoutExec(f.rt.calls), "started and not stopped")
	assert.Contains(t, f.out.String(), "The crannog of main runs in the background. Stop it with: crannog stop main")
	assert.Contains(t, f.out.String(), "Connect over SSH: crannog ssh main, or ssh "+tg.names.Container+
		" from other tools (needs Include ~/.crannog/ssh/config in ~/.ssh/config)\n")
	assert.Equal(t, tg.names.Remote, testgit.Git(t, f.dir, "remote"), "the remote stays while the crannog runs")
	files := sessionFiles(t, tg)
	require.Len(t, files, 1)
	assert.Contains(t, files[0], "command: "+waitCommand+"\ndetached: true\nstarted_at: ")
	list, err := f.m.List()
	require.NoError(t, err)
	assert.Equal(t, []Info{{Branch: "main", Status: runtime.StateRunning, Sessions: 1, SSHHost: tg.names.Container}}, list,
		"self-repair leaves the crannog alone")

	// Another session next to the detached one: crannog warns as next to
	// any session, and the crannog keeps running when that session ends.
	f.rt.calls = nil
	f.out.Reset()
	code, err := f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	assert.Equal(t, 0, code)
	assert.Contains(t, f.out.String(), "1 session is already running in this crannog.")
	assert.Empty(t, withoutExec(f.rt.calls))
	n, err := tg.dir.sessions(false)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the detached session")

	f.out.Reset()
	require.NoError(t, f.m.Stop(""), f.out.String())
	assert.Empty(t, f.prompt.asked, "stop asks nothing")
	assert.Equal(t, []string{"stop " + tg.names.Container + " " + tg.names.Proxy}, withoutExec(f.rt.calls))
	assert.Contains(t, f.out.String(), "Waiting for 1 session in the crannog of main to end")
	assert.Contains(t, f.out.String(), "Stopping the crannog of main")
	assert.Empty(t, testgit.Git(t, f.dir, "remote"))
	n, err = tg.dir.sessions(false)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Stopping a stopped crannog is no error.
	f.rt.calls = nil
	f.out.Reset()
	require.NoError(t, f.m.Stop("main"))
	assert.Equal(t, "The crannog of main is not running\n", f.out.String())
	assert.Empty(t, f.rt.calls)
}

// With a command after --, the detached session runs it instead of
// waiting. session.command is not used.
func TestDetachedCheckoutRunsTheCommand(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.existing("main", defaultSetup(t))

	require.NoError(t, f.m.CheckoutDetached("main", []string{"npm", "run", "dev server"}), f.out.String())
	assert.Equal(t, []string{"detach " + tg.names.Container + " npm run 'dev server'"}, f.rt.callsWith("detach"))
}

// A detached session that cannot be started leaves no session behind, and
// the crannog stops again.
func TestDetachedCheckoutThatFailsToStart(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.existing("main", defaultSetup(t))
	f.rt.detachErr = assert.AnError

	require.ErrorIs(t, f.m.CheckoutDetached("", nil), assert.AnError)
	assert.NotContains(t, f.out.String(), "runs in the background")
	assert.Equal(t, runtime.StateStopped, f.rt.containers[tg.names.Container].State)
	assert.Equal(t, runtime.StateStopped, f.rt.containers[tg.names.Proxy].State)
	n, err := tg.dir.sessions(false)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// When the command of a detached session ends by itself, nobody is left
// to stop the crannog: the next crannog command does (ADR 0019).
func TestDetachedSessionThatEndsByItself(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.existing("main", defaultSetup(t))
	require.NoError(t, f.m.CheckoutDetached("", []string{"true"}), f.out.String())
	f.rt.endDetached()
	assert.Equal(t, runtime.StateRunning, f.rt.containers[tg.names.Container].State)

	list, err := f.m.List()
	require.NoError(t, err)
	assert.Equal(t, []Info{{Branch: "main", Status: runtime.StateStopped, SSHHost: tg.names.Container}}, list)
	assert.Contains(t, f.out.String(), "The crannog of main runs without a session.")
	assert.Empty(t, testgit.Git(t, f.dir, "remote"))
}

// stop ends every session without a confirmation, which would be an error
// here, without a terminal. It needs the crannog, but not its branch,
// which was deleted here.
func TestStopEndsSessions(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("gone", defaultSetup(t))
	f.rt.containers[tg.names.Proxy].State = runtime.StateRunning
	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	sess, _, err := tg.dir.registerSession("true", false)
	require.NoError(t, err)
	defer sess.Unlock()
	f.rt.execHook = func(name string, args []string) error {
		if args[2] == signalScript && args[4] == "term" {
			sess.Unlock()
		}
		return nil
	}

	require.NoError(t, f.m.Stop("gone"), f.out.String())
	assert.Empty(t, f.prompt.asked)
	assert.Contains(t, f.out.String(), "Waiting for 1 session in the crannog of gone to end")
	assert.Equal(t, []string{"stop " + tg.names.Container + " " + tg.names.Proxy}, withoutExec(f.rt.calls))
}

func TestStopWithoutCrannog(t *testing.T) {
	f := newFixture(t)
	assert.ErrorContains(t, f.m.Stop(""), "there is no crannog for main")
	assert.ErrorContains(t, f.m.Stop("feature/x"), "there is no crannog for feature/x")
}

// A crannog that runs without a session, e.g. after the command of a
// detached session ended by itself, is stopped by stop itself, which says
// so once.
func TestStopOfACrannogWithoutSessions(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	other := f.existing("other", defaultSetup(t))
	for _, c := range []string{tg.names.Proxy, tg.names.Container, other.names.Proxy, other.names.Container} {
		f.rt.containers[c].State = runtime.StateRunning
	}

	require.NoError(t, f.m.Stop("main"), f.out.String())
	assert.Equal(t, "The crannog of other runs without a session.\nStopping the crannog of other\nStopping the crannog of main\n", f.out.String(),
		"self-repair stops the other crannog, stop this one")
	assert.Equal(t, runtime.StateStopped, f.rt.containers[tg.names.Container].State)
	assert.Equal(t, runtime.StateStopped, f.rt.containers[other.names.Container].State)
}
