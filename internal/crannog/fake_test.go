package crannog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/repository"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/testgit"
	"github.com/hettiger/crannog/internal/ui"
)

// fakeRuntime simulates the container CLI.
type fakeRuntime struct {
	containers map[string]*runtime.Container
	images     map[string]bool
	calls      []string
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{containers: map[string]*runtime.Container{}, images: map[string]bool{}}
}

func (f *fakeRuntime) record(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *fakeRuntime) List(prefix string) ([]runtime.Container, error) {
	var list []runtime.Container
	for name, c := range f.containers {
		if strings.HasPrefix(name, prefix) {
			list = append(list, *c)
		}
	}
	return list, nil
}

func (f *fakeRuntime) ImageExists(name string) (bool, error) { return f.images[name], nil }

func (f *fakeRuntime) Build(tag, dir string) error {
	f.record("build %s", tag)
	f.images[tag] = true
	return nil
}

func (f *fakeRuntime) Pull(ref string) error {
	f.record("pull %s", ref)
	f.images[ref] = true
	return nil
}

func (f *fakeRuntime) CreateNetwork(name string) error {
	f.record("network create %s", name)
	return nil
}
func (f *fakeRuntime) DeleteNetwork(name string) error {
	f.record("network delete %s", name)
	return nil
}
func (f *fakeRuntime) CreateVolume(name string) error { f.record("volume create %s", name); return nil }
func (f *fakeRuntime) DeleteVolume(name string) error { f.record("volume delete %s", name); return nil }

func (f *fakeRuntime) Create(s runtime.Spec) error {
	f.record("create %s %s", s.Name, s.Image)
	c := &runtime.Container{Name: s.Name, State: runtime.StateStopped}
	for i, n := range s.Networks {
		c.Networks = append(c.Networks, runtime.Attachment{Network: n, IPv4: fmt.Sprintf("192.168.70.%d", i+2)})
	}
	f.containers[s.Name] = c
	return nil
}

func (f *fakeRuntime) RunOnce(s runtime.Spec) error {
	f.record("run %s", strings.Join(s.Args, " "))
	return nil
}

func (f *fakeRuntime) Start(name string) error {
	f.record("start %s", name)
	f.containers[name].State = runtime.StateRunning
	return nil
}

func (f *fakeRuntime) Stop(names ...string) error {
	f.record("stop %s", strings.Join(names, " "))
	for _, n := range names {
		f.containers[n].State = runtime.StateStopped
	}
	return nil
}

func (f *fakeRuntime) Delete(names ...string) error {
	f.record("delete %s", strings.Join(names, " "))
	for _, n := range names {
		delete(f.containers, n)
	}
	return nil
}

func (f *fakeRuntime) Exec(name string, o runtime.ExecOptions, args ...string) ([]byte, error) {
	if c := f.containers[name]; c == nil || c.State != runtime.StateRunning {
		return nil, fmt.Errorf("container %s is not running", name)
	}
	f.record("exec %s %s", name, args[0])
	return nil, nil
}

func (f *fakeRuntime) ExecCommand(name string, o runtime.ExecOptions, args ...string) *exec.Cmd {
	return exec.Command("true")
}

// fixture is a repository with a manager on a fake runtime.
type fixture struct {
	t       *testing.T
	dir     string
	m       *Manager
	rt      *fakeRuntime
	prompt  *fakePrompter
	out     *bytes.Buffer
	hostEnv map[string]string
}

type fakePrompter struct {
	terminal bool
	answer   bool
	asked    []string
}

func (p *fakePrompter) Interactive() bool { return p.terminal }
func (p *fakePrompter) Ask(q string) (bool, error) {
	p.asked = append(p.asked, q)
	return p.answer, nil
}
func (p *fakePrompter) Pick(string, []string) (string, error) { return "", nil }

func newFixture(t *testing.T) *fixture {
	dir := testgit.Init(t)
	repo, err := repository.Open(dir)
	require.NoError(t, err)
	f := &fixture{
		t:       t,
		dir:     dir,
		rt:      newFakeRuntime(),
		prompt:  &fakePrompter{},
		out:     &bytes.Buffer{},
		hostEnv: map[string]string{},
	}
	userHome := t.TempDir()
	f.m = &Manager{
		Home:    home.Home{Dir: userHome + "/.crannog"},
		Repo:    repo,
		Runtime: f.rt,
		UI:      &ui.UI{Prompter: f.prompt, Out: f.out},
		LookupEnv: func(k string) (string, bool) {
			v, ok := f.hostEnv[k]
			return v, ok
		},
		UserHome: userHome,
	}
	return f
}

func (f *fixture) target(branch string) *target {
	t, err := f.m.target(branch)
	require.NoError(f.t, err)
	return t
}
