// Package crannog builds, starts, stops and destroys a repository's
// crannogs and runs sessions in them.
package crannog

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/google/renameio/v2"

	"github.com/hettiger/crannog/internal/config"
	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/openssh"
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
	CrannogHome home.CrannogHome
	Repo        *repository.Repository
	Runtime     runtime.Runtime
	UI          *ui.UI
	// LookupEnv reads the host's environment for interpolation.
	LookupEnv func(string) (string, bool)
	// UserHome is the user's home directory, for ~/ in mounts.
	UserHome string
	// Executable is the path of the running crannog binary, which the
	// ProxyCommand of the SSH causeway runs.
	Executable string
	// ContainerCLI is the path of the container CLI, which the
	// ProxyCommand passes on: apps started from the Dock have a short PATH
	// without it.
	ContainerCLI string
	// SSHClient runs ssh on the host for crannog ssh.
	SSHClient SSHClient
}

// SSHClient runs the system ssh with the user's SSH configuration
// (package openssh).
type SSHClient interface {
	// Config returns the options ssh would use for host, by option name
	// in lower case.
	Config(ctx context.Context, host string) (map[string][]string, error)
	// Replace replaces this process with ssh to host.
	Replace(host string, o openssh.Options) error
}

// New returns a manager for the repository of the current directory.
func New(u *ui.UI) (*Manager, error) {
	containerCLI, err := runtime.LookPathCLI()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repo, err := repository.Open(cwd)
	if err != nil {
		return nil, err
	}
	h, err := home.OpenCrannogHome()
	if err != nil {
		return nil, err
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &Manager{
		CrannogHome:  h,
		Repo:         repo,
		Runtime:      runtime.CLI{Progress: u.Out, Path: containerCLI},
		UI:           u,
		LookupEnv:    os.LookupEnv,
		UserHome:     userHome,
		Executable:   exe,
		ContainerCLI: containerCLI,
		SSHClient:    openssh.Client{},
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
		dir:    newHostDir(m.CrannogHome, m.Repo.ID, s),
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
	return setup.Store{Root: m.CrannogHome.Root, Dir: home.ApprovedPath(m.Repo.ID)}
}

// vars returns the variables for interpolating the crannog's config.yml.
func (m *Manager) vars(t *target) config.Vars {
	return config.Vars{
		Crannog: map[string]string{
			config.AgentsDir: m.CrannogHome.Abs(home.Agents),
			config.CacheDir:  m.CrannogHome.Abs(home.Cache),
			config.CADir:     t.dir.CADir(),
			config.RepoDir:   m.Repo.PathInCrannog(),
			config.ProxyURL:  config.ProxyURLValue,
		},
		Host: m.LookupEnv,
	}
}

// approvedConfig reads the config.yml of an approved setup as written,
// without interpolation: what a session needs to start does not depend on
// values from the host (ADR 0025).
func (m *Manager) approvedConfig(hash string) (setup.Setup, *config.Config, error) {
	s, err := m.store().Get(hash)
	if err != nil {
		return nil, nil, err
	}
	c, err := config.Parse(s[setup.ConfigFile].Data)
	if err != nil {
		return nil, nil, err
	}
	return s, c, nil
}

// loadApproved reads an approved setup and its config.yml, interpolated
// with the host's current environment, for a build or rebuild.
func (m *Manager) loadApproved(t *target, hash string) (setup.Setup, *config.Config, error) {
	s, c, err := m.approvedConfig(hash)
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
	list, err := m.list(n.Container)
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

// remoteURL returns the URL of a remote, or "" if it does not exist.
func (m *Manager) remoteURL(name string) (string, error) {
	return git.RemoteURL(m.Repo.Dir, name)
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

// recordPath writes the repository's git common dir, from which its id is
// derived, to repositories/<id>/path (ADR 0021).
func (m *Manager) recordPath() error {
	r, err := m.CrannogHome.OpenDir(home.RepositoryPath(m.Repo.ID), true)
	if err != nil {
		return err
	}
	defer r.Close()
	want := []byte(m.Repo.CommonDir + "\n")
	if got, err := r.ReadFile(home.PathFile); err == nil && bytes.Equal(got, want) {
		return nil
	}
	return renameio.WriteFile(home.PathFile, want, 0o644, renameio.WithRoot(r))
}

// addRemote adds the crannog's remote, unless it exists. A remote of that
// name with another URL is the user's: an error, see checkRemote.
func (m *Manager) addRemote(n naming.Names) error {
	url, err := m.remoteURL(n.Remote)
	switch {
	case err != nil:
		return err
	case url == n.URL:
		return nil
	case url != "":
		return fmt.Errorf("a remote named %s already exists (%s); rename or remove it", n.Remote, url)
	}
	return git.AddRemote(m.Repo.Dir, n.Remote, n.URL)
}

// removeRemote removes the crannog's remote, but never a remote of that
// name with another URL or other fetch mappings.
func (m *Manager) removeRemote(n naming.Names) error {
	return git.RemoveRemote(m.Repo.Dir, n.Remote, n.URL)
}
