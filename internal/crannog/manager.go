// Package crannog builds, starts, stops and destroys a repository's
// crannogs and runs sessions in them.
package crannog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/hettiger/crannog/internal/config"
	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/repository"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/setup"
	"github.com/hettiger/crannog/internal/slug"
	"github.com/hettiger/crannog/internal/ui"
)

// AgentUser is the user agents and sessions run as (ADR 0011).
const AgentUser = "agent"

// Manager handles the crannogs of one repository.
type Manager struct {
	Home    home.Home
	Repo    *repository.Repository
	Runtime runtime.Runtime
	UI      *ui.UI
	// LookupEnv reads the host's environment for interpolation.
	LookupEnv func(string) (string, bool)
	// UserHome is the user's home directory, for ~/ in mounts.
	UserHome string
}

// New returns a manager for the repository of the current directory.
func New(u *ui.UI) (*Manager, error) {
	if _, err := exec.LookPath("container"); err != nil {
		return nil, errors.New("the container CLI of Apple Containers is not in PATH (https://github.com/apple/container)")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repo, err := repository.Open(cwd)
	if err != nil {
		return nil, err
	}
	h, err := home.Default()
	if err != nil {
		return nil, err
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Manager{
		Home:      h,
		Repo:      repo,
		Runtime:   runtime.CLI{Progress: u.Out},
		UI:        u,
		LookupEnv: os.LookupEnv,
		UserHome:  userHome,
	}, nil
}

// target is the crannog of one start branch.
type target struct {
	branch string
	names  naming.Names
	dir    HostDir
}

// target returns the crannog of branch. It refuses if the crannog's host
// directory names another branch (ADR 0017).
func (m *Manager) target(branch string) (*target, error) {
	s := slug.BranchSlug(branch)
	t := &target{
		branch: branch,
		names:  naming.New(m.Repo.ID8(), s),
		dir:    newHostDir(m.Home, m.Repo.ID, s),
	}
	recorded, err := t.dir.Branch()
	if err != nil {
		return nil, err
	}
	if recorded != "" && recorded != branch {
		return nil, fmt.Errorf("the crannog directory %s belongs to branch %q, not %q", t.dir.Dir, recorded, branch)
	}
	return t, nil
}

func (m *Manager) store() setup.Store {
	return setup.Store{Root: m.Home.Root, Dir: home.ApprovedPath(m.Repo.ID)}
}

// vars returns the variables for interpolating the crannog's config.yml.
func (m *Manager) vars(t *target) config.Vars {
	return config.Vars{
		Crannog: map[string]string{
			config.AgentsDir: m.Home.Abs(home.Agents),
			config.CacheDir:  m.Home.Abs(home.Cache),
			config.CADir:     t.dir.CADir(),
			config.RepoDir:   m.Repo.PathInCrannog(),
			config.ProxyURL:  config.ProxyURLValue,
		},
		Host: m.LookupEnv,
	}
}

// loadApproved reads an approved setup and its config.yml, interpolated.
func (m *Manager) loadApproved(t *target, hash string) (setup.Setup, *config.Config, error) {
	s, err := m.store().Get(hash)
	if err != nil {
		return nil, nil, err
	}
	c, err := config.Parse(s[setup.ConfigFile].Data)
	if err != nil {
		return nil, nil, err
	}
	c, err = c.Expand(m.vars(t), m.UserHome)
	if err != nil {
		return nil, nil, err
	}
	return s, c, nil
}

// status is the runtime state of a crannog's containers.
type status struct {
	crannog *runtime.Container
	proxy   *runtime.Container
}

func (s status) running() bool {
	return s.crannog != nil && s.crannog.State == runtime.StateRunning
}

func (s status) proxyRunning() bool {
	return s.proxy != nil && s.proxy.State == runtime.StateRunning
}

func (m *Manager) status(n naming.Names) (status, error) {
	list, err := m.Runtime.List(context.Background(), n.Container)
	if err != nil {
		return status{}, err
	}
	var s status
	for i := range list {
		switch list[i].Name {
		case n.Container:
			s.crannog = &list[i]
		case n.Proxy:
			s.proxy = &list[i]
		}
	}
	return s, nil
}

// checkHelper fails early if git cannot find the remote helper.
func checkHelper() error {
	if _, err := exec.LookPath("git-remote-crannog"); err != nil {
		return errors.New("git-remote-crannog is not in PATH; install it next to crannog")
	}
	return nil
}

// remoteURL returns the URL of a remote, or "" if it does not exist.
func (m *Manager) remoteURL(name string) (string, error) {
	url, err := m.Repo.Git("config", "--get", "remote."+name+".url")
	if git.ExitCode(err) == 1 {
		return "", nil
	}
	return url, err
}

// checkRemote fails if a remote with the crannog's remote name but another
// URL exists: it is the user's, and crannog refuses to start (ADR 0015).
func (m *Manager) checkRemote(n naming.Names) error {
	url, err := m.remoteURL(n.Remote)
	if err != nil {
		return err
	}
	if url != "" && url != n.URL {
		return fmt.Errorf("a remote named %s already exists (%s); rename or remove it", n.Remote, url)
	}
	return nil
}

// addRemote adds the crannog's remote.
func (m *Manager) addRemote(n naming.Names) error {
	if err := m.checkRemote(n); err != nil {
		return err
	}
	url, err := m.remoteURL(n.Remote)
	if err != nil || url == n.URL {
		return err
	}
	if _, err := m.Repo.Git("remote", "add", n.Remote, n.URL); err != nil {
		return err
	}
	_, err = m.Repo.Git("config", "remote."+n.Remote+".followRemoteHEAD", "never")
	return err
}

// removeRemote removes the crannog's remote, but never a remote of that
// name with another URL.
func (m *Manager) removeRemote(n naming.Names) error {
	url, err := m.remoteURL(n.Remote)
	if err != nil || url != n.URL {
		return err
	}
	mappings, err := m.Repo.Git("config", "--get-all", "remote."+n.Remote+".fetch")
	if err != nil && git.ExitCode(err) != 1 {
		return err
	}
	want := "+refs/heads/*:refs/remotes/" + n.Remote + "/*"
	if err != nil || mappings != want {
		return fmt.Errorf("refusing to remove remote %s: unexpected fetch mappings", n.Remote)
	}
	_, err = m.Repo.Git("remote", "remove", n.Remote)
	return err
}
