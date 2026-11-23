package crannog

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/session"
	"github.com/hettiger/crannog/internal/setup"
	"github.com/hettiger/crannog/internal/shell"
)

// sessionScript starts the session command in the agent user's login
// shell (ADR 0025).
const sessionScript = `shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
exec "${shell:-/bin/sh}" -l -c "$1"`

// Checkout opens a session in the crannog of a start branch, building or
// starting the crannog as needed (ADR 0027). args replaces session.command.
// It returns the session's exit code.
func (m *Manager) Checkout(branchArg string, args []string) (int, error) {
	if err := git.CheckHelper(); err != nil {
		return 0, err
	}
	branch, err := m.Repo.StartBranch(branchArg)
	if err != nil {
		return 0, err
	}
	t, err := m.target(branch)
	if err != nil {
		return 0, err
	}
	m.SelfRepair()

	src, err := m.readSource(branch)
	if err != nil {
		return 0, err
	}
	if err := m.recordPath(); err != nil {
		return 0, err
	}
	lock, err := m.lock(t)
	if err != nil {
		return 0, err
	}
	defer lock.Unlock()

	err = m.ensureRunning(t, src)
	var command string
	if err == nil {
		command, err = m.sessionCommand(t, args)
	}
	if err == nil {
		err = m.prepareSession(t)
	}
	var sess *session.Lock
	var name string
	if err == nil {
		// The session file stays locked while the session runs, also by
		// the container CLI if crannog is killed (ADR 0019).
		sess, name, err = t.dir.registerSession()
	}
	if err != nil {
		// Without a host directory there is no crannog to stop: a first
		// checkout that did not build removed it.
		if exists, eerr := t.dir.exists(); eerr != nil || exists {
			if serr := m.stopIdle(t); serr != nil {
				m.UI.Printf("Warning: %v\n", serr)
			}
		}
		return 0, err
	}
	lock.Unlock()

	code, runErr := m.runSession(t, sess, command)

	sess.Unlock()
	_ = t.dir.removeSession(name)
	if err := m.endSession(t); err != nil {
		m.UI.Printf("Warning: %v\n", err)
	}
	return code, runErr
}

// sessionCommand returns the command a session runs: the arguments after
// --, quoted, or session.command of the crannog's copy.
func (m *Manager) sessionCommand(t *target, args []string) (string, error) {
	if len(args) > 0 {
		return shell.Join(args), nil
	}
	hash, err := t.dir.Hash()
	if err != nil {
		return "", err
	}
	_, cfg, err := m.approvedConfig(hash)
	if err != nil {
		return "", err
	}
	if cfg.Session.Command == "" {
		return "", fmt.Errorf("%s has no session.command; pass a command after --", setup.ConfigFile)
	}
	return cfg.Session.Command, nil
}

// ensureRunning builds, updates or starts the crannog so a session can
// run. The caller holds the lock.
func (m *Manager) ensureRunning(t *target, src *source) error {
	hash, err := t.dir.Hash()
	if err != nil {
		return err
	}
	if hash == "" {
		// A recorded branch without a hash: the remains of an interrupted
		// build (ADR 0021). A fresh crannog has neither, and nothing to
		// remove: containers, a network or volumes with its names are not
		// from a build crannog knows of, so the user removes them. A first
		// checkout that does not build leaves no host directory behind.
		recorded, err := t.dir.Branch()
		if err != nil {
			return err
		}
		if recorded != "" {
			if err := m.remove(t); err != nil {
				return err
			}
		} else {
			found, err := m.leftovers(t)
			if err != nil {
				return err
			}
			if len(found) > 0 {
				err := fmt.Errorf("cannot check out %s: the following resources already exist\n  %s\nRemove them, then run crannog checkout again.",
					t.branch, strings.Join(found, "\n  "))
				return errors.Join(err, t.dir.removeAll())
			}
		}
		_, hash, err := m.review(t, src, nil, 0, true, false)
		if err != nil {
			return errors.Join(err, t.dir.removeAll())
		}
		if err := t.dir.WriteBranch(t.branch); err != nil {
			return err
		}
		return m.create(t, hash)
	}

	current, err := m.store().Get(hash)
	if err != nil {
		return err
	}
	// Containers, the network and the state volume can be made again, the
	// work volume cannot (ADR 0021).
	o, err := m.objects(t)
	if err != nil {
		return err
	}
	if !o.work {
		return errWorkVolumeMissing(t)
	}
	incomplete := m.noteMissing(t, o)
	sessions, err := t.dir.sessions(true)
	if err != nil {
		return err
	}
	c, next, err := m.review(t, src, current, sessions, true, incomplete)
	if err != nil {
		return err
	}
	switch c {
	case changeReload:
		if err := m.reload(t, next); err != nil {
			return err
		}
	case changeRebuild:
		return m.rebuild(t, next, true)
	}
	return m.start(t, o.status)
}

// lock takes the crannog's lock, waiting for other crannog processes.
func (m *Manager) lock(t *target) (*session.Lock, error) {
	return t.dir.lock(m.waiting(t))
}

// waiting returns the function that says that crannog waits for the lock.
func (m *Manager) waiting(t *target) func() {
	return func() {
		m.UI.Printf("Waiting for another crannog command on the crannog of %s\n", t.branch)
	}
}

// runSession runs the command in the crannog as the agent user in the
// repository and returns its exit code.
func (m *Manager) runSession(t *target, sess *session.Lock, command string) (int, error) {
	o := runtime.ExecOptions{
		User:        AgentUser,
		Workdir:     m.Repo.PathInCrannog(),
		Interactive: true,
		TTY:         m.UI.Interactive(),
	}
	cmd := m.Runtime.ExecCommand(t.names.Container, o, "sh", "-c", sessionScript, "crannog", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{sess.File()}

	// crannog outlives the session to clean up. The terminal sends Ctrl-C
	// to the session itself; termination requests are passed on.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-signals:
				if s == syscall.SIGTERM || s == syscall.SIGHUP {
					_ = cmd.Process.Signal(s)
				}
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	return exitCode(err)
}

func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, err
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal()), nil
	}
	return exitErr.ExitCode(), nil
}

// signalScript ends the programs of the sessions without killing them, so
// they restore the terminals of their sessions. The runtime starts the
// crannog's main process and every command it executes in a process
// session of its own. The script leaves the session of the main process
// alone, whose end would stop the crannog, and its own.
//
// With term, it sends SIGTERM to all other processes. With hup, it hangs
// up those that ignore SIGTERM (bit 0x4000 of SigIgn), as interactive
// shells do, once they have no child process left: a program in the
// foreground of a shell restores the terminal before its session ends.
const signalScript = `sid() { sed 's/.*) //' "/proc/$1/stat" 2>/dev/null | cut -d' ' -f4; }
main=$(sid 1)
self=$(sid $$)
parents=" $(sed -n 's/^PPid:[[:space:]]*//p' /proc/[0-9]*/status 2>/dev/null | tr '\n' ' ')"
for d in /proc/[0-9]*; do
  pid=${d#/proc/}
  s=$(sid "$pid")
  [ -n "$s" ] && [ "$s" != "$main" ] && [ "$s" != "$self" ] || continue
  if [ "$1" = term ]; then
    kill -TERM "$pid" 2>/dev/null
    continue
  fi
  case "$parents" in *" $pid "*) continue ;; esac
  ign=$(sed -n 's/^SigIgn:[[:space:]]*//p' "$d/status" 2>/dev/null)
  [ -n "$ign" ] || continue
  [ $((0x${ign#"${ign%????}"} & 0x4000)) -ne 0 ] && kill -HUP "$pid" 2>/dev/null
done
true`

// sessionPoll is how often endSessions counts the sessions.
var sessionPoll = 500 * time.Millisecond

// endSessions asks the programs in the crannog to end and waits until no
// session runs, without a timeout: nothing is deleted before, so Ctrl-C
// leaves the crannog as it is (ADR 0019). The caller holds the lock.
func (m *Manager) endSessions(t *target) error {
	n, err := t.dir.sessions(true)
	if err != nil || n == 0 {
		return err
	}
	signal := func(mode string) error {
		_, err := m.exec(t.names.Container, runtime.ExecOptions{User: AgentUser}, "sh", "-c", signalScript, "sh", mode)
		return err
	}
	title := fmt.Sprintf("Waiting for %s in the crannog of %s to end (Ctrl-C keeps the crannog as it is)", plural(n, "session", "sessions"), t.branch)
	err = m.UI.WaitFor(title, func(ctx context.Context) error {
		if err := signal("term"); err != nil {
			return err
		}
		for {
			n, err := t.dir.sessions(true)
			if err != nil || n == 0 {
				return err
			}
			if err := signal("hup"); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sessionPoll):
			}
		}
	})
	if err != nil {
		return fmt.Errorf("ending the sessions in the crannog of %s: %w; nothing was deleted", t.branch, err)
	}
	return nil
}

// endSession stops the crannog when the last session ended (ADR 0019). A
// crannog destroyed while the session ran has nothing left to stop; its
// host directory is not created again.
func (m *Manager) endSession(t *target) error {
	lock, err := t.dir.lockExisting(m.waiting(t))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, session.ErrReplaced) {
		m.UI.Printf("The crannog of %s was destroyed while this session ran\n", t.branch)
		return nil
	}
	if err != nil {
		return err
	}
	defer lock.Unlock()
	return m.stopIdle(t)
}

// stopIdle stops the crannog if no session runs. The caller holds the
// lock.
func (m *Manager) stopIdle(t *target) error {
	n, err := t.dir.sessions(true)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return m.stop(t)
}
