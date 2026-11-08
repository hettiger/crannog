// Package repository finds the user's repository on the host (ADR 0018,
// ADR 0021).
package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/hettiger/crannog/internal/git"
)

// WorkDir is the mount point of a crannog's work volume (ADR 0018).
const WorkDir = "/work"

// ErrDetached is returned by CurrentBranch on a detached HEAD.
var ErrDetached = errors.New("HEAD is detached; name a branch")

// Repository is the user's git repository on the host. All of its
// worktrees count as one repository.
type Repository struct {
	// Dir is the top level of the current worktree. It is empty when the
	// repository was located from a git dir only (see Locate).
	Dir string
	// CommonDir is the absolute path of the git common dir.
	CommonDir string
	// MainWorktree is the absolute path of the main worktree.
	MainWorktree string
	// ID is the sha256 of CommonDir in hex.
	ID string
}

// Open finds the repository of the worktree that contains dir.
func Open(dir string) (*Repository, error) {
	top, err := git.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not in a git worktree: %w", err)
	}
	r, err := Locate(top)
	if err != nil {
		return nil, err
	}
	r.Dir = top
	return r, nil
}

// Locate finds the repository from dir without requiring a worktree, as
// the remote helper does: git runs it with GIT_DIR set.
func Locate(dir string) (*Repository, error) {
	common, err := git.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}
	list, err := git.Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	main, err := mainWorktree(list)
	if err != nil {
		return nil, err
	}
	return &Repository{CommonDir: common, MainWorktree: main, ID: ID(common)}, nil
}

// mainWorktree returns the path of the first worktree in the output of
// git worktree list --porcelain, which is the main worktree.
func mainWorktree(porcelain string) (string, error) {
	block, _, _ := strings.Cut(porcelain, "\n\n")
	var dir string
	for _, line := range strings.Split(block, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			dir = p
		}
		if line == "bare" {
			return "", errors.New("bare repositories are not supported")
		}
	}
	if dir == "" {
		return "", errors.New("cannot find the main worktree")
	}
	return dir, nil
}

// ID returns the repository id for a git common dir.
func ID(commonDir string) string {
	sum := sha256.Sum256([]byte(commonDir))
	return hex.EncodeToString(sum[:])
}

// ID8 returns the first eight characters of the id, used in runtime names.
func (r *Repository) ID8() string { return r.ID[:8] }

// PathInCrannog returns the path of the repository inside a crannog:
// /work followed by the main worktree's path on the host.
func (r *Repository) PathInCrannog() string {
	return path.Join(WorkDir, r.MainWorktree)
}

// Git runs git in the current worktree.
func (r *Repository) Git(args ...string) (string, error) {
	return git.Run(r.Dir, args...)
}

// CurrentBranch returns the branch checked out in the current worktree.
func (r *Repository) CurrentBranch() (string, error) {
	b, err := r.Git("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		if git.ExitCode(err) == 1 {
			return "", ErrDetached
		}
		return "", err
	}
	return b, nil
}

// HasBranch reports whether branch is a local branch.
func (r *Repository) HasBranch(branch string) (bool, error) {
	_, err := r.Git("rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		if git.ExitCode(err) == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// StartBranch resolves the [branch] argument of a command: the given
// branch, or the checked-out one. It must be a local branch.
func (r *Repository) StartBranch(arg string) (string, error) {
	branch := arg
	if branch == "" {
		b, err := r.CurrentBranch()
		if err != nil {
			return "", err
		}
		branch = b
	}
	ok, err := r.HasBranch(branch)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%q is not a local branch", branch)
	}
	return branch, nil
}
