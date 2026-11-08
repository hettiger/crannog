package crannog

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/session"
	"github.com/hettiger/crannog/internal/setup"
	"github.com/hettiger/crannog/internal/shell"
)

// sessionScript starts the session command in the agent user's login
// shell (ADR 0019).
const sessionScript = `shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
exec "${shell:-/bin/sh}" -l -c "$1"`

// Checkout opens a session in the crannog of a start branch, building or
// starting the crannog as needed (ADR 0016). args replaces session.command.
// It returns the session's exit code.
func (m *Manager) Checkout(branchArg string, args []string) (int, error) {
	if err := checkHelper(); err != nil {
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
		// the container CLI if crannog is killed (ADR 0025).
		sess, name, err = t.dir.registerSession()
	}
	if err != nil {
		if serr := m.stopIdle(t); serr != nil {
			m.UI.Printf("Warning: %v\n", serr)
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
	_, cfg, err := m.loadApproved(t, hash)
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
		// Remains of an interrupted build, if any.
		if err := m.remove(t); err != nil {
			return err
		}
		_, hash, err := m.review(t, src, nil, 0, true)
		if err != nil {
			_ = t.dir.clear()
			return err
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
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	if st.crannog == nil || st.proxy == nil {
		hint := "the work volume may still contain work; crannog destroy deletes it"
		if planChange(current, src.setup) == changeRebuild {
			hint = fmt.Sprintf("crannog apply %s recreates them with this setup and keeps the work volume; crannog destroy deletes it", shell.Quote(t.branch))
		}
		return fmt.Errorf("the containers of the crannog of %s are missing; %s", t.branch, hint)
	}
	sessions, err := t.dir.sessions(true)
	if err != nil {
		return err
	}
	c, next, err := m.review(t, src, current, sessions, true)
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
	return m.start(t, st)
}

// lock takes the crannog's lock, waiting for other crannog processes.
func (m *Manager) lock(t *target) (*session.Lock, error) {
	return t.dir.lock(func() {
		m.UI.Printf("Waiting for another crannog command on the crannog of %s\n", t.branch)
	})
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

// endSession stops the crannog when the last session ended (ADR 0014).
func (m *Manager) endSession(t *target) error {
	lock, err := m.lock(t)
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
