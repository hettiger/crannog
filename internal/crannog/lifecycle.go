package crannog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/renameio/v2"

	"github.com/hettiger/crannog/internal/config"
	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/instructions"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/setup"
)

// Conventions of the proxy causeway that crannog relies on; the default
// preset's iron-proxy.yaml spells them out (ADR 0023).
const (
	proxyDirInProxy  = "/etc/iron-proxy"
	proxyConfigFile  = "iron-proxy.yaml"
	proxyHealthURL   = "http://127.0.0.1:9090/healthz"
	proxyReloadURL   = "http://127.0.0.1:9092/v1/reload"
	proxyMemory      = "256M"
	proxyHostName    = "crannog-proxy"
	readyMarker      = "/tmp/ready"
	pollInterval     = 250 * time.Millisecond
	managementKeyEnv = "IRON_MANAGEMENT_API_KEY"
	caCertFile       = "ca.crt"
	caKeyFile        = "ca.key"
)

// commandTimeout bounds every command crannog runs itself in a crannog or
// against the runtime: the readiness probes, the scripts before a session,
// the destroy check, the proxy reload and container list (ADR 0020). Image
// builds, pulls, the repository transfer and sessions are not bounded.
const commandTimeout = 60 * time.Second

// bounded runs f with a context that ends after commandTimeout. It is the
// one place that applies the timeout.
func bounded[T any](f func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	return f(ctx)
}

// exec runs a command in a container, bounded.
func (m *Manager) exec(name string, o runtime.ExecOptions, args ...string) ([]byte, error) {
	return bounded(func(ctx context.Context) ([]byte, error) { return m.Runtime.Exec(ctx, name, o, args...) })
}

// list lists the containers with a name prefix, bounded.
func (m *Manager) list(prefix string) ([]runtime.Container, error) {
	return bounded(func(ctx context.Context) ([]runtime.Container, error) { return m.Runtime.List(ctx, prefix) })
}

// create builds a new crannog from an approved setup and starts it. On
// failure, it removes everything it created (the image stays).
func (m *Manager) create(t *target, hash string) (err error) {
	defer func() {
		if err != nil {
			m.UI.Printf("Building the crannog failed, removing what was created.\n")
			if rerr := m.remove(t); rerr != nil {
				m.UI.Printf("Warning: %v\n", rerr)
			}
		}
	}()

	approved, cfg, err := m.loadApproved(t, hash)
	if err != nil {
		return err
	}
	if err := t.dir.mkdirs(filepath.Join(proxyDir, "secrets"), caDir, sessionsDir); err != nil {
		return err
	}
	if err := m.prepareHost(cfg); err != nil {
		return err
	}
	if err := m.prepareImages(approved, hash, cfg); err != nil {
		return err
	}

	if err := writeCA(t); err != nil {
		return err
	}
	if err := m.writeProxyConfig(t, approved); err != nil {
		return err
	}

	m.UI.Printf("Creating the crannog\n")
	if err := m.Runtime.CreateNetwork(t.names.Network); err != nil {
		return err
	}
	for _, v := range []string{t.names.WorkVolume, t.names.StateVolume} {
		if err := m.Runtime.CreateVolume(v); err != nil {
			return err
		}
	}
	if err := m.createContainers(t, approved, cfg); err != nil {
		return err
	}
	if err := m.start(t, status{}); err != nil {
		return err
	}

	m.UI.Printf("Transferring the repository\n")
	repo := m.Repo.PathInCrannog()
	if _, err := m.exec(t.names.Container, runtime.ExecOptions{User: AgentUser},
		"sh", "-c", `set -e; mkdir -p "$1"; git init -q -b "$2" "$1"; git -C "$1" config receive.denyCurrentBranch updateInstead`,
		"sh", repo, t.branch); err != nil {
		return err
	}
	// All branches arrive as origin/* and only the start branch as a local
	// branch, like in a fresh clone (ADR 0015).
	if err := m.transferRepository(t); err != nil {
		return err
	}
	return t.dir.WriteHash(hash)
}

// transferRepository pushes directly to the crannog, not a configured push URL.
func (m *Manager) transferRepository(t *target) error {
	push := git.Command(m.Repo.Dir, "push", "--quiet", t.names.URL,
		"refs/heads/*:refs/remotes/origin/*",
		"refs/tags/*:refs/tags/*",
		"refs/heads/"+t.branch+":refs/heads/"+t.branch)
	push.Stdout = m.UI.Out
	push.Stderr = m.UI.Out
	if err := push.Run(); err != nil {
		return fmt.Errorf("initial transfer of the repository: %w", err)
	}
	return nil
}

// prepareHost creates the directories crannog provides on the host. It
// never writes into them: crannogs can write there (ADR 0014).
func (m *Manager) prepareHost(cfg *config.Config) error {
	for _, d := range []string{home.Agents, home.Cache} {
		if err := hostfs.MkdirAll(m.Home.Root, d); err != nil {
			return err
		}
	}
	// The only access outside the anchors (ADR 0013): approved mount
	// sources are checked to exist, the runtime mounts them.
	for _, mt := range cfg.Mounts {
		if _, err := os.Stat(mt.Source); err != nil {
			return fmt.Errorf("mount source %s: %w", mt.Source, err)
		}
	}
	return nil
}

// prepareImages builds the crannog's image from the approved setup if it is
// missing and pulls the proxy image.
func (m *Manager) prepareImages(approved setup.Setup, hash string, cfg *config.Config) error {
	image := approved.ImageName()
	ok, err := m.Runtime.ImageExists(image)
	if err != nil {
		return err
	}
	if !ok {
		m.UI.Printf("Building image %s\n", image)
		ctx := m.Home.Abs(filepath.Join(home.ApprovedPath(m.Repo.ID), hash, setup.ContainerDir))
		if err := m.Runtime.Build(image, ctx); err != nil {
			return err
		}
	}
	ok, err = m.Runtime.ImageExists(cfg.Proxy.Image)
	if err != nil {
		return err
	}
	if !ok {
		m.UI.Printf("Pulling %s\n", cfg.Proxy.Image)
		return m.Runtime.Pull(cfg.Proxy.Image)
	}
	return nil
}

// createContainers creates the proxy and the crannog, both stopped.
func (m *Manager) createContainers(t *target, approved setup.Setup, cfg *config.Config) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	if err := m.Runtime.Create(runtime.Spec{
		Name:     t.names.Proxy,
		Image:    cfg.Proxy.Image,
		Memory:   proxyMemory,
		Networks: []string{"default", t.names.Network},
		// Only the host writes the proxy's directory (ADR 0014).
		Mounts: []runtime.Mount{{Source: t.dir.ProxyDir(), Target: proxyDirInProxy, ReadOnly: true}},
		// The management API listens on loopback in the proxy only;
		// crannog reads the key from the proxy's environment when it
		// reloads, so it is not stored anywhere else.
		Env:  []string{managementKeyEnv + "=" + hex.EncodeToString(key)},
		Args: []string{"-config", path.Join(proxyDirInProxy, proxyConfigFile)},
	}); err != nil {
		return err
	}

	mounts := []runtime.Mount{
		{Source: t.names.WorkVolume, Target: "/work"},
		{Source: t.names.StateVolume, Target: "/state"},
	}
	for _, mt := range cfg.Mounts {
		mounts = append(mounts, runtime.Mount{Source: mt.Source, Target: mt.Target, ReadOnly: mt.ReadOnly})
	}
	return m.Runtime.Create(runtime.Spec{
		Name:     t.names.Container,
		Image:    approved.ImageName(),
		CPUs:     cfg.Resources.CPUs,
		Memory:   cfg.Resources.Memory,
		Networks: []string{t.names.Network},
		Mounts:   mounts,
		Env:      cfg.EnvList(),
	})
}

// writeProxyConfig gives the proxy its copy of iron-proxy.yaml.
func (m *Manager) writeProxyConfig(t *target, approved setup.Setup) error {
	return writeProxyFile(t, approved[setup.ProxyFile].Data)
}

// readProxyFile and writeProxyFile access the proxy's iron-proxy.yaml.
func readProxyFile(t *target) ([]byte, error) {
	proxy, err := t.dir.open(proxyDir, false)
	if err != nil {
		return nil, err
	}
	defer proxy.Close()
	return proxy.ReadFile(proxyConfigFile)
}

func writeProxyFile(t *target, data []byte) error {
	proxy, err := t.dir.open(proxyDir, false)
	if err != nil {
		return err
	}
	defer proxy.Close()
	return renameio.WriteFile(proxyConfigFile, data, 0o644, renameio.WithRoot(proxy))
}

// start starts the proxy and the crannog if they are not running, waits
// until both are ready and adds the remote. The caller holds the lock.
func (m *Manager) start(t *target, st status) error {
	if err := m.checkRemote(t.names); err != nil {
		return err
	}
	if !st.running() {
		m.UI.Printf("Starting the crannog of %s\n", t.branch)
	}
	if !st.proxyRunning() {
		if err := m.Runtime.Start(t.names.Proxy); err != nil {
			return err
		}
	}
	if err := m.waitFor(t.names.Proxy, "the proxy to become healthy", func(ctx context.Context) bool {
		// -T ends the probe in the proxy, which keeps running when the
		// deadline kills the CLI.
		_, err := m.Runtime.Exec(ctx, t.names.Proxy, runtime.ExecOptions{}, "wget", "-q", "-T", "5", "-O", "/dev/null", proxyHealthURL)
		return err == nil
	}); err != nil {
		return err
	}
	if !st.running() {
		if err := m.Runtime.Start(t.names.Container); err != nil {
			return err
		}
	}
	// The entrypoint writes the boot id once the crannog is prepared
	// (ADR 0020).
	if err := m.waitFor(t.names.Container, "the readiness marker "+readyMarker+" with the current boot id", func(ctx context.Context) bool {
		_, err := m.Runtime.Exec(ctx, t.names.Container, runtime.ExecOptions{User: "root"}, "sh", "-c",
			`[ "$(cat `+readyMarker+` 2>/dev/null)" = "$(cat /proc/sys/kernel/random/boot_id)" ]`)
		return err == nil
	}); err != nil {
		return fmt.Errorf("%w; the image's entrypoint must write /proc/sys/kernel/random/boot_id to %s once the crannog is prepared", err, readyMarker)
	}
	return m.addRemote(t.names)
}

// waitFor polls ready until it holds, the container stopped or the
// command timeout passed. The probes share the deadline, so a hanging
// probe ends with it.
func (m *Manager) waitFor(container, what string, ready func(context.Context) bool) error {
	_, err := bounded(func(ctx context.Context) (struct{}, error) {
		for i := 0; ; i++ {
			if ready(ctx) {
				return struct{}{}, nil
			}
			if i%8 == 7 {
				list, err := m.Runtime.List(ctx, container)
				if err == nil && !hasRunning(list, container) {
					return struct{}{}, fmt.Errorf("%s stopped while waiting for %s; see container logs %s", container, what, container)
				}
			}
			select {
			case <-ctx.Done():
				return struct{}{}, fmt.Errorf("timed out after %s waiting for %s", commandTimeout, what)
			case <-time.After(pollInterval):
			}
		}
	})
	return err
}

func hasRunning(list []runtime.Container, name string) bool {
	for _, c := range list {
		if c.Name == name && c.State == runtime.StateRunning {
			return true
		}
	}
	return false
}

// stop stops the crannog and its proxy and removes the remote. The caller
// holds the lock.
func (m *Manager) stop(t *target) error {
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	var names []string
	if st.running() {
		names = append(names, t.names.Container)
	}
	if st.proxyRunning() {
		names = append(names, t.names.Proxy)
	}
	if len(names) > 0 {
		m.UI.Printf("Stopping the crannog of %s\n", t.branch)
		if err := m.Runtime.Stop(names...); err != nil {
			return err
		}
	}
	return m.removeRemote(t.names)
}

// rebuild replaces the proxy and the crannog with containers from another
// approved setup. Volumes, network, CA and the host directory stay. The new
// image is built first; a failing build leaves the old containers in place
// (ADR 0022). With start, the crannog is started afterwards; otherwise it
// stays stopped until the next session.
func (m *Manager) rebuild(t *target, hash string, start bool) error {
	approved, cfg, err := m.loadApproved(t, hash)
	if err != nil {
		return err
	}
	if err := m.prepareHost(cfg); err != nil {
		return err
	}
	if err := m.prepareImages(approved, hash, cfg); err != nil {
		return err
	}
	m.UI.Printf("Rebuilding the crannog of %s\n", t.branch)
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	var old []string
	if st.crannog != nil {
		old = append(old, t.names.Container)
	}
	if st.proxy != nil {
		old = append(old, t.names.Proxy)
	}
	if len(old) > 0 {
		if err := m.Runtime.Delete(old...); err != nil {
			return err
		}
	}
	if err := m.removeRemote(t.names); err != nil {
		return err
	}
	if err := m.writeProxyConfig(t, approved); err != nil {
		return err
	}
	if err := m.createContainers(t, approved, cfg); err != nil {
		return err
	}
	if err := t.dir.WriteHash(hash); err != nil {
		return err
	}
	if start {
		return m.start(t, status{})
	}
	return nil
}

// reload gives the proxy a new iron-proxy.yaml and reloads it if it runs.
// Running sessions continue. If the proxy rejects the file, the old one is
// restored.
func (m *Manager) reload(t *target, hash string) error {
	approved, err := m.store().Get(hash)
	if err != nil {
		return err
	}
	old, err := readProxyFile(t)
	if err != nil {
		return err
	}
	if err := m.writeProxyConfig(t, approved); err != nil {
		return err
	}
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	if st.proxyRunning() {
		m.UI.Printf("Reloading the proxy of %s\n", t.branch)
		if err := m.reloadProxy(t); err != nil {
			if rerr := writeProxyFile(t, old); rerr == nil {
				_ = m.reloadProxy(t)
			}
			return fmt.Errorf("the proxy rejected the new %s, the old one stays: %w", setup.ProxyFile, err)
		}
	}
	return t.dir.WriteHash(hash)
}

func (m *Manager) reloadProxy(t *target) error {
	_, err := m.exec(t.names.Proxy, runtime.ExecOptions{}, "sh", "-c",
		`wget -q -O - --post-data= --header "Authorization: Bearer $`+managementKeyEnv+`" `+proxyReloadURL)
	return err
}

// prepareSession runs before every session: it maps crannog-proxy to the
// proxy's current address in /etc/hosts (ADR 0010), mirrors the git
// author of the host repository (ADR 0016) and writes the managed
// instructions block (ADR 0029).
func (m *Manager) prepareSession(t *target) error {
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	if st.proxy == nil {
		return errors.New("the proxy does not exist")
	}
	ip := st.proxy.IPv4On(t.names.Network)
	if ip == "" {
		return fmt.Errorf("the proxy has no address on %s", t.names.Network)
	}
	if _, err := m.exec(t.names.Container, runtime.ExecOptions{User: "root"}, "sh", "-c", hostsScript, "sh", ip, proxyHostName); err != nil {
		return fmt.Errorf("writing /etc/hosts: %w", err)
	}

	args := []string{"sh", "-c", authorScript, "sh", m.Repo.PathInCrannog()}
	for _, key := range []string{"user.name", "user.email"} {
		v, err := m.Repo.Git("config", key)
		switch {
		case err == nil:
			args = append(args, "set", v)
		case git.ExitCode(err) == 1:
			m.UI.Printf("Warning: git %s is not set for this repository on the host; commits in the crannog will fail.\n", key)
			args = append(args, "unset", "")
		default:
			return err
		}
	}
	if _, err := m.exec(t.names.Container, runtime.ExecOptions{User: AgentUser}, args...); err != nil {
		return fmt.Errorf("setting the git author: %w", err)
	}
	return m.writeInstructions(t)
}

// writeInstructions updates the managed instructions block in each agent's
// instruction file. It runs in the crannog, so the host never writes into
// the agent homes (ADR 0014). Without a writable mount of the agent homes
// there is nothing to update.
func (m *Manager) writeInstructions(t *target) error {
	hash, err := t.dir.Hash()
	if err != nil {
		return err
	}
	_, cfg, err := m.loadApproved(t, hash)
	if err != nil {
		return err
	}
	dir := agentsMount(cfg, m.Home.Abs(home.Agents))
	if dir == "" {
		return nil
	}
	for _, a := range instructions.Agents {
		file := path.Join(dir, a.Name, a.File)
		o := runtime.ExecOptions{User: AgentUser}
		content, err := m.exec(t.names.Container, o, "sh", "-c", readInstructionsScript, "sh", file)
		if err != nil {
			return fmt.Errorf("reading %s: %w", file, err)
		}
		updated := instructions.Update(string(content), instructions.ManagedBlock)
		if updated == string(content) {
			continue
		}
		o.Stdin = strings.NewReader(updated)
		if _, err := m.exec(t.names.Container, o, "sh", "-c", writeInstructionsScript, "sh", file); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}
	return nil
}

// agentsMount returns where the agent homes are mounted writable in the
// crannog, or "".
func agentsMount(cfg *config.Config, agents string) string {
	for _, mt := range cfg.Mounts {
		if filepath.Clean(mt.Source) == agents && !mt.ReadOnly {
			return mt.Target
		}
	}
	return ""
}

// readInstructionsScript creates the agent home and prints the instruction
// file $1, if it exists.
const readInstructionsScript = `mkdir -p "${1%/*}" && if [ -e "$1" ]; then cat "$1"; fi`

// writeInstructionsScript replaces the instruction file $1 with stdin.
const writeInstructionsScript = `cat > "$1.crannog-tmp" && mv -f "$1.crannog-tmp" "$1"`

// hostsScript writes "<ip> <name>" to /etc/hosts if it is missing or
// differs. It rewrites the file in place: the runtime provides it.
const hostsScript = `grep -qx "$1 $2" /etc/hosts && exit 0
grep -v " $2\$" /etc/hosts > /tmp/hosts.crannog || true
echo "$1 $2" >> /tmp/hosts.crannog
cat /tmp/hosts.crannog > /etc/hosts
rm -f /tmp/hosts.crannog`

// authorScript sets or unsets user.name and user.email in the repository
// in the crannog. Arguments: repo, then set|unset and the value per key.
const authorScript = `repo=$1
if [ "$2" = set ]; then git -C "$repo" config user.name "$3"; else git -C "$repo" config --unset-all user.name || :; fi
if [ "$4" = set ]; then git -C "$repo" config user.email "$5"; else git -C "$repo" config --unset-all user.email || :; fi`

// remove deletes everything of a crannog except the image and the host
// directory's lock: containers, network, volumes, remote and host state.
// If a runtime object or the remote cannot be deleted, the host state
// stays, so the next checkout tries again.
func (m *Manager) remove(t *target) error {
	var errs []error
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	var containers []string
	if st.crannog != nil {
		containers = append(containers, t.names.Container)
	}
	if st.proxy != nil {
		containers = append(containers, t.names.Proxy)
	}
	if len(containers) > 0 {
		errs = append(errs, m.Runtime.Delete(containers...))
	}
	errs = append(errs, ignoreMissing(m.Runtime.DeleteNetwork(t.names.Network)))
	for _, v := range []string{t.names.WorkVolume, t.names.StateVolume} {
		errs = append(errs, ignoreMissing(m.Runtime.DeleteVolume(v)))
	}
	errs = append(errs, m.removeRemote(t.names))
	if err := errors.Join(errs...); err != nil {
		return err
	}
	return t.dir.clear()
}

// ignoreMissing ignores errors about objects that do not exist.
func ignoreMissing(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil
	}
	return err
}
