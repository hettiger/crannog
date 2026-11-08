package crannog

import (
	"errors"
	"fmt"

	"github.com/hettiger/crannog/internal/config"
	"github.com/hettiger/crannog/internal/preset"
	"github.com/hettiger/crannog/internal/repository"
	"github.com/hettiger/crannog/internal/setup"
)

// source is the setup a crannog should use and where it comes from.
type source struct {
	setup  setup.Setup
	config *config.Config
	// desc is printed, e.g. "from feature/x (commit a1b2c3d)".
	desc string
	// preset is set if the setup comes from a preset, which needs no
	// approval (ADR 0026).
	preset bool
}

// readSource reads the setup of a start branch: from the working tree if
// the branch is checked out in the current worktree, otherwise from the
// branch's latest commit, and the preset default if that has none
// (ADR 0023). It prints the source.
func (m *Manager) readSource(branch string) (*source, error) {
	current, err := m.Repo.CurrentBranch()
	if err != nil && !errors.Is(err, repository.ErrDetached) {
		return nil, err
	}
	changed, err := setup.HasChanges(m.Repo.Dir)
	if err != nil {
		return nil, err
	}

	src := &source{}
	if current == branch {
		src.setup, err = setup.ReadWorktree(m.Repo.Dir)
		src.desc = "from working tree"
		if changed {
			src.desc += " (uncommitted changes)"
		}
	} else {
		var commit string
		commit, err = m.Repo.Git("rev-parse", "--short", "refs/heads/"+branch)
		if err != nil {
			return nil, err
		}
		src.setup, err = setup.ReadCommit(m.Repo.Dir, "refs/heads/"+branch)
		src.desc = fmt.Sprintf("from %s (commit %s)", branch, commit)
	}
	if err != nil {
		return nil, err
	}

	if len(src.setup) == 0 {
		p, err := preset.Find(m.Home, preset.Default)
		if err != nil {
			return nil, err
		}
		src.setup, err = p.Load()
		if err != nil {
			return nil, err
		}
		src.preset = true
		src.desc = "from preset " + preset.Default
		if p.User {
			src.desc += " (" + p.Dir + ")"
		}
	}

	m.UI.Printf("Using %s/ %s\n", setup.Dir, src.desc)
	if changed && current != branch {
		m.UI.Printf("Note: the uncommitted changes to %s/ in this worktree are not used; %s is not checked out here.\n", setup.Dir, branch)
	}

	if err := src.setup.Validate(); err != nil {
		return nil, err
	}
	src.config, err = config.Parse(src.setup[setup.ConfigFile].Data)
	if err != nil {
		return nil, err
	}
	return src, nil
}
