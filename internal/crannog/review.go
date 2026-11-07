package crannog

import (
	"fmt"
	"strings"

	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/setup"
)

// change is what a new setup means for a crannog.
type change int

const (
	changeNone change = iota
	// changeReload: only iron-proxy.yaml differs; the proxy reloads it.
	changeReload
	// changeRebuild: the containers are replaced.
	changeRebuild
	// changeCreate: the crannog does not exist yet.
	changeCreate
)

// planChange compares the crannog's copy (nil for a new crannog) with the
// new setup (ADR 0016).
func planChange(current, next setup.Setup) change {
	switch {
	case current == nil:
		return changeCreate
	case current.Equal(next):
		return changeNone
	case current.Without(setup.ProxyFile).Equal(next.Without(setup.ProxyFile)):
		return changeReload
	default:
		return changeRebuild
	}
}

// question is the one question crannog asks before a change (ADR 0018).
type question int

const (
	askNothing question = iota
	// askApproval: a setup approval; --yes does not answer it.
	askApproval
	// askRebuild: a confirmation that running sessions end.
	askRebuild
	// askSecondSession: a confirmation to start another session.
	askSecondSession
)

// decide returns the question to ask. approved tells whether the new
// content was approved before; fromPreset whether it comes from a preset.
func decide(c change, approved, fromPreset bool, sessions int, checkout bool) question {
	switch {
	case c != changeNone && !approved && !fromPreset:
		return askApproval
	case c == changeRebuild && sessions > 0:
		return askRebuild
	case checkout && sessions > 0:
		return askSecondSession
	default:
		return askNothing
	}
}

// review decides what the new setup means for the crannog and asks the
// user if needed. current is the crannog's copy, nil for a new crannog. It
// stores approved and preset setups as approved setups and returns the
// change and the hash of the setup to use.
func (m *Manager) review(t *target, src *source, current setup.Setup, sessions int, checkout bool) (change, string, error) {
	c := planChange(current, src.setup)
	store := m.store()
	hash := src.setup.Hash()
	approved := true
	if c != changeNone {
		var err error
		if approved, err = store.Has(hash); err != nil {
			return 0, "", err
		}
	}

	note := ""
	switch {
	case sessions > 0 && c == changeRebuild:
		note = fmt.Sprintf("%s running in this crannog end when it is rebuilt.", plural(sessions, "session", "sessions"))
	case sessions > 0 && checkout:
		note = fmt.Sprintf("%s already running in this crannog.", plural(sessions, "session is", "sessions are"))
	}

	switch decide(c, approved, src.preset, sessions, checkout) {
	case askApproval:
		base := current
		if base == nil {
			base = m.latestApproved()
		}
		if err := m.showDiff(base, src.setup); err != nil {
			return 0, "", err
		}
		if vars := src.config.HostVariables(); len(vars) > 0 {
			m.UI.Printf("This setup reads these variables from your environment: %s\n", strings.Join(vars, ", "))
		}
		if note != "" {
			m.UI.Printf("%s\n", note)
		}
		if err := m.UI.AskSetupApproval(fmt.Sprintf("Approve this setup for the crannog of %s?", t.branch)); err != nil {
			return 0, "", err
		}
		if _, err := store.Add(src.setup); err != nil {
			return 0, "", err
		}
		if err := store.SetLatest(hash); err != nil {
			return 0, "", err
		}
		return c, hash, nil

	case askRebuild:
		if err := m.showDiff(current, src.setup); err != nil {
			return 0, "", err
		}
		m.UI.Printf("%s\n", note)
		if err := m.UI.AskConfirmation(fmt.Sprintf("Rebuild the crannog of %s?", t.branch)); err != nil {
			return 0, "", err
		}

	case askSecondSession:
		if !m.UI.Yes && !m.UI.Interactive() {
			m.UI.Printf("Warning: %s\n", note)
		} else if err := m.UI.AskConfirmation(note + " Start another one?"); err != nil {
			return 0, "", err
		}
	}

	if src.preset && !approved {
		if _, err := store.Add(src.setup); err != nil {
			return 0, "", err
		}
		if err := store.SetLatest(hash); err != nil {
			return 0, "", err
		}
	}
	return c, hash, nil
}

// latestApproved returns the most recently approved setup, or nil.
func (m *Manager) latestApproved() setup.Setup {
	store := m.store()
	latest, err := store.Latest()
	if err != nil || latest == "" {
		return nil
	}
	s, err := store.Get(latest)
	if err != nil {
		return nil
	}
	return s
}

// showDiff prints the difference between two setups with git diff
// --no-index. A nil base shows everything as new.
func (m *Manager) showDiff(base, next setup.Setup) error {
	tmp, err := hostfs.NewTempDir("diff")
	if err != nil {
		return err
	}
	defer tmp.Remove()
	if base == nil {
		base = setup.Setup{}
	}
	if err := base.Write(tmp.Root, "approved"); err != nil {
		return err
	}
	if err := next.Write(tmp.Root, setup.Dir); err != nil {
		return err
	}

	args := []string{"-c", "core.quotepath=false"}
	if !m.UI.Interactive() {
		args = append(args, "--no-pager")
	}
	args = append(args, "diff", "--no-index", "--no-prefix", "--", "approved", setup.Dir)
	cmd := git.Command(tmp.Dir, args...)
	cmd.Stdout = m.UI.Out
	cmd.Stderr = m.UI.Out
	if err := cmd.Run(); err != nil && git.ExitCode(err) != 1 {
		return fmt.Errorf("git diff: %w", err)
	}
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
