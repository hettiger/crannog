// Package runtime calls Apple Containers' container CLI (ADR 0006).
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Container states as reported by container list.
const (
	StateRunning = "running"
	StateStopped = "stopped"
)

// Container is a container as reported by container list.
type Container struct {
	Name     string
	State    string
	Networks []Attachment
}

// Attachment is a container's interface on a network.
type Attachment struct {
	Network string
	// IPv4 is the address without prefix length.
	IPv4 string
}

// IPv4On returns the container's address on a network, or "".
func (c Container) IPv4On(network string) string {
	for _, a := range c.Networks {
		if a.Network == network {
			return a.IPv4
		}
	}
	return ""
}

// Mount is a bind mount or a named volume.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// Spec describes a container to create.
type Spec struct {
	Name     string
	Image    string
	CPUs     string
	Memory   string
	Networks []string
	Mounts   []Mount
	Env      []string
	// Args are passed to the image's entrypoint.
	Args []string
}

// args returns the options and arguments of container create or run.
func (s Spec) args() []string {
	var a []string
	if s.Name != "" {
		a = append(a, "--name", s.Name)
	}
	if s.CPUs != "" {
		a = append(a, "--cpus", s.CPUs)
	}
	if s.Memory != "" {
		a = append(a, "--memory", s.Memory)
	}
	for _, n := range s.Networks {
		a = append(a, "--network", n)
	}
	for _, m := range s.Mounts {
		v := m.Source + ":" + m.Target
		if m.ReadOnly {
			v += ":ro"
		}
		a = append(a, "--volume", v)
	}
	for _, e := range s.Env {
		a = append(a, "--env", e)
	}
	a = append(a, s.Image)
	return append(a, s.Args...)
}

// ExecOptions configure container exec.
type ExecOptions struct {
	User        string
	Workdir     string
	Interactive bool
	TTY         bool
	Env         []string
	// Stdin is passed to the command by Exec, which then implies
	// Interactive.
	Stdin io.Reader
}

func (o ExecOptions) args() []string {
	var a []string
	if o.Interactive || o.Stdin != nil {
		a = append(a, "--interactive")
	}
	if o.TTY {
		a = append(a, "--tty")
	}
	if o.User != "" {
		a = append(a, "--user", o.User)
	}
	if o.Workdir != "" {
		a = append(a, "--workdir", o.Workdir)
	}
	for _, e := range o.Env {
		a = append(a, "--env", e)
	}
	return a
}

// Runtime is the container runtime crannog uses. CLI implements it; tests
// use fakes.
type Runtime interface {
	// List returns all containers, running or not, whose names start with
	// prefix.
	List(ctx context.Context, prefix string) ([]Container, error)
	ImageExists(name string) (bool, error)
	Build(tag, contextDir string) error
	Pull(ref string) error
	CreateNetwork(name string) error
	DeleteNetwork(name string) error
	CreateVolume(name string) error
	DeleteVolume(name string) error
	// Create creates a container without starting it.
	Create(s Spec) error
	Start(name string) error
	Stop(names ...string) error
	// Delete removes containers, running or not.
	Delete(names ...string) error
	// Exec runs a command in a running container and returns its stdout.
	Exec(ctx context.Context, name string, o ExecOptions, args ...string) ([]byte, error)
	// ExecCommand returns a command that runs args in a container, for
	// sessions and the remote helper.
	ExecCommand(name string, o ExecOptions, args ...string) *exec.Cmd
}

// CLI runs the container CLI. Output of long-running commands (build,
// pull) goes to Progress.
type CLI struct {
	Progress io.Writer
}

// Error is a failed container command.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("container %s: %v", strings.Join(e.Args, " "), e.Err)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode returns the exit code of a failed command, or -1.
func ExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func (c CLI) progress() io.Writer {
	if c.Progress == nil {
		return os.Stderr
	}
	return c.Progress
}

func (c CLI) run(args ...string) ([]byte, error) {
	return c.runContext(context.Background(), args...)
}

func (c CLI) runContext(ctx context.Context, args ...string) ([]byte, error) {
	return c.runInput(ctx, nil, args...)
}

func (CLI) runInput(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "container", args...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		return out, &Error{Args: args, Stderr: cleanStderr(stderr.String()), Err: err}
	}
	return out, nil
}

// runVisible runs a command with its output on the progress writer.
func (c CLI) runVisible(args ...string) error {
	cmd := exec.Command("container", args...)
	cmd.Stdout = c.progress()
	cmd.Stderr = c.progress()
	if err := cmd.Run(); err != nil {
		return &Error{Args: args, Err: err}
	}
	return nil
}

// cleanStderr drops the progress lines ("[1/6] ...") of the container CLI.
func cleanStderr(s string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "[") {
			continue
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "; ")
}

func (c CLI) List(ctx context.Context, prefix string) ([]Container, error) {
	out, err := c.runContext(ctx, "list", "--all", "--format", "json")
	if err != nil {
		return nil, err
	}
	all, err := parseList(out)
	if err != nil {
		return nil, err
	}
	var list []Container
	for _, ct := range all {
		if strings.HasPrefix(ct.Name, prefix) {
			list = append(list, ct)
		}
	}
	return list, nil
}

// listEntry is the part of container list --format json crannog reads.
type listEntry struct {
	Configuration struct {
		ID string `json:"id"`
	} `json:"configuration"`
	Status struct {
		State    string `json:"state"`
		Networks []struct {
			Network     string `json:"network"`
			IPv4Address string `json:"ipv4Address"`
		} `json:"networks"`
	} `json:"status"`
}

func parseList(data []byte) ([]Container, error) {
	var entries []listEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("cannot parse the output of container list: %w", err)
	}
	list := make([]Container, 0, len(entries))
	for _, e := range entries {
		ct := Container{Name: e.Configuration.ID, State: e.Status.State}
		for _, n := range e.Status.Networks {
			ip, _, _ := strings.Cut(n.IPv4Address, "/")
			ct.Networks = append(ct.Networks, Attachment{Network: n.Network, IPv4: ip})
		}
		list = append(list, ct)
	}
	return list, nil
}

func (c CLI) ImageExists(name string) (bool, error) {
	_, err := c.run("image", "inspect", name)
	if err == nil {
		return true, nil
	}
	if ExitCode(err) > 0 {
		return false, nil
	}
	return false, err
}

func (c CLI) Build(tag, contextDir string) error {
	return c.runVisible("build", "--tag", tag, contextDir)
}

func (c CLI) Pull(ref string) error {
	return c.runVisible("image", "pull", ref)
}

func (c CLI) CreateNetwork(name string) error {
	_, err := c.run("network", "create", "--internal", name)
	return err
}

func (c CLI) DeleteNetwork(name string) error {
	_, err := c.run("network", "delete", name)
	return err
}

func (c CLI) CreateVolume(name string) error {
	_, err := c.run("volume", "create", name)
	return err
}

func (c CLI) DeleteVolume(name string) error {
	_, err := c.run("volume", "delete", name)
	return err
}

func (c CLI) Create(s Spec) error {
	_, err := c.run(append([]string{"create"}, s.args()...)...)
	return err
}

func (c CLI) Start(name string) error {
	_, err := c.run("start", name)
	return err
}

func (c CLI) Stop(names ...string) error {
	_, err := c.run(append([]string{"stop"}, names...)...)
	return err
}

func (c CLI) Delete(names ...string) error {
	_, err := c.run(append([]string{"delete", "--force"}, names...)...)
	return err
}

func (c CLI) Exec(ctx context.Context, name string, o ExecOptions, args ...string) ([]byte, error) {
	a := append([]string{"exec"}, o.args()...)
	a = append(a, name)
	return c.runInput(ctx, o.Stdin, append(a, args...)...)
}

func (CLI) ExecCommand(name string, o ExecOptions, args ...string) *exec.Cmd {
	a := append([]string{"exec"}, o.args()...)
	a = append(a, name)
	return exec.Command("container", append(a, args...)...)
}
