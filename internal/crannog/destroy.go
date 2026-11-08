package crannog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/runtime"
)

// losses is what destroying a crannog would lose.
type losses struct {
	// branches are the crannog's branches with commits not reachable from
	// the user's own refs, with the number of such commits.
	branches []lostBranch
	// worktrees are worktrees in the crannog with uncommitted changes.
	worktrees []string
}

type lostBranch struct {
	name    string
	commits int
}

// Destroy destroys the crannog of a start branch after showing what would
// be lost and asking for confirmation (ADR 0015, ADR 0027). The
// branch does not need to exist anymore.
func (m *Manager) Destroy(branchArg string) error {
	branch := branchArg
	if branch == "" {
		b, err := m.Repo.CurrentBranch()
		if err != nil {
			return err
		}
		branch = b
	}
	t, err := m.target(branch)
	if err != nil {
		return err
	}
	exists, err := t.dir.exists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("there is no crannog for %s", branch)
	}
	if err := checkHelper(); err != nil {
		return err
	}
	m.SelfRepair()

	lock, err := m.lock(t)
	if err != nil {
		return err
	}
	defer lock.Unlock()

	sessions, err := t.dir.sessions(true)
	if err != nil {
		return err
	}
	l, checkErr := m.losses(t)
	if checkErr != nil {
		m.UI.Printf("Warning: cannot check what would be lost: %v\n", checkErr)
		m.UI.Printf("Unmerged commits and uncommitted changes in the crannog may be lost.\n")
	} else {
		printLosses(m, t, l)
	}
	if sessions > 0 {
		m.UI.Printf("%s running in this crannog end.\n", plural(sessions, "session", "sessions"))
		m.UI.Printf("Running sessions can create more work after this check; that work will also be deleted.\n")
	}
	if err := m.UI.AskConfirmation(fmt.Sprintf("Destroy the crannog of %s?", branch)); err != nil {
		// The check may have started the crannog.
		if serr := m.stopIdle(t); serr != nil {
			m.UI.Printf("Warning: %v\n", serr)
		}
		return err
	}

	m.UI.Printf("Destroying the crannog of %s\n", branch)
	if err := m.remove(t); err != nil {
		return err
	}
	return t.dir.removeAll()
}

// losses starts the crannog if needed, fetches its branches directly via
// its URL and finds what would be lost (ADR 0015).
func (m *Manager) losses(t *target) (losses, error) {
	var l losses
	st, err := m.status(t.names)
	if err != nil {
		return l, err
	}
	if st.crannog == nil || st.proxy == nil {
		return l, errors.New("the crannog's containers are missing")
	}
	if !st.running() || !st.proxyRunning() {
		if err := m.start(t, st); err != nil {
			return l, err
		}
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return l, err
	}
	refs := "refs/crannog/destroy/" + t.names.Slug + "/" + hex.EncodeToString(random) + "/"
	existing, err := m.Repo.Git("for-each-ref", "--format=%(refname)", refs)
	if err != nil {
		return l, err
	}
	if existing != "" {
		return l, fmt.Errorf("temporary ref namespace %s is not empty", refs)
	}
	defer m.deleteRefs(refs)
	if _, err := m.Repo.Git("fetch", "--quiet", "--no-write-fetch-head", "--no-tags", "--no-prune", "--no-prune-tags", "--no-auto-maintenance", t.names.URL,
		"+refs/heads/*:"+refs+"*"); err != nil {
		return l, err
	}
	list, err := m.Repo.Git("for-each-ref", "--format=%(refname)", refs)
	if err != nil {
		return l, err
	}
	for _, ref := range lines(list) {
		n, err := m.Repo.Git("rev-list", "--count", ref, "--not",
			"--exclude=refs/crannog/*", "--exclude=refs/remotes/"+t.names.Remote+"/*", "--all")
		if err != nil {
			return l, err
		}
		count, err := strconv.Atoi(n)
		if err != nil {
			return l, err
		}
		if count > 0 {
			l.branches = append(l.branches, lostBranch{name: strings.TrimPrefix(ref, refs), commits: count})
		}
	}

	out, err := m.Runtime.Exec(context.Background(), t.names.Container, runtime.ExecOptions{User: AgentUser}, "sh", "-c", dirtyScript, "sh", m.Repo.PathInCrannog())
	if err != nil {
		return l, fmt.Errorf("checking worktrees: %w", err)
	}
	l.worktrees = lines(string(out))
	return l, nil
}

// dirtyScript lists the worktrees with uncommitted changes, untracked
// files included.
const dirtyScript = `git -C "$1" worktree list --porcelain | sed -n 's/^worktree //p' |
while IFS= read -r w; do
  [ -n "$(git -C "$w" status --porcelain 2>/dev/null)" ] && echo "$w"
done
true`

func printLosses(m *Manager, t *target, l losses) {
	if len(l.branches) == 0 && len(l.worktrees) == 0 {
		m.UI.Printf("At the time of this check, all branch commits of the crannog of %s were in your repository and no uncommitted changes were found.\n", t.branch)
		return
	}
	if len(l.branches) > 0 {
		m.UI.Printf("Branches with commits that are not in your repository:\n")
		for _, b := range l.branches {
			m.UI.Printf("  %s (%s)\n", b.name, plural(b.commits, "commit", "commits"))
		}
	}
	if len(l.worktrees) > 0 {
		m.UI.Printf("Worktrees with uncommitted changes:\n")
		for _, w := range l.worktrees {
			m.UI.Printf("  %s\n", w)
		}
	}
}

// deleteRefs deletes the refs below prefix.
func (m *Manager) deleteRefs(prefix string) {
	list, err := m.Repo.Git("for-each-ref", "--format=delete %(refname)", prefix)
	if err != nil || list == "" {
		return
	}
	cmd := git.Command(m.Repo.Dir, "update-ref", "--no-deref", "--stdin")
	cmd.Stdin = strings.NewReader(list + "\n")
	_ = cmd.Run()
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
