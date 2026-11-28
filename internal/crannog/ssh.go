package crannog

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"golang.org/x/crypto/ssh"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/repository"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/shell"
	"github.com/hettiger/crannog/internal/slug"
)

// The SSH causeway: ssh on the host reaches sshd in a crannog through a
// ProxyCommand that runs crannog ssh-proxy, which pipes sshd -i through
// container exec. No port is involved.

// Names of the SSH causeway's files.
const (
	// sshDir is the directory below ~/.crannog and below a crannog's host
	// directory.
	sshDir = "ssh"
	// sshConfigFile is the config the user includes, in ~/.crannog/ssh, and
	// a crannog's Host block, in its host directory's ssh/.
	sshConfigFile  = "config"
	clientKeyFile  = "id_ed25519"
	knownHostsFile = "known_hosts"
	hostKeyFile    = "ssh_host_ed25519_key"
	// sshDirInCrannog is where the crannog keeps sshd's files. /state is
	// the agent user's, of this crannog only.
	sshDirInCrannog = "/state/ssh"
)

// prepareSSH writes what the SSH causeway needs on the host and in the
// running crannog: the keys, the crannog's Host block and known_hosts, and
// in the crannog the host key and authorized_keys. The host generates both
// keys, so nothing from the crannog reaches the user's SSH files. The
// crannog writes its files itself (ADR 0014). The caller holds the lock.
func (m *Manager) prepareSSH(t *target) error {
	if err := m.writeSSHInclude(); err != nil {
		return err
	}
	client, err := ensureClientKey(m.CrannogHome)
	if err != nil {
		return err
	}
	dir, err := t.dir.open("", false)
	if err != nil {
		return err
	}
	defer dir.Close()
	r, err := privateDir(dir, sshDir)
	if err != nil {
		return err
	}
	defer r.Close()
	hostKey, hostKeyPEM, err := ensureKey(r, hostKeyFile, t.names.Container)
	if err != nil {
		return err
	}
	if err := writeIfChanged(r, knownHostsFile, []byte(knownHostsLine(t.names.Container, hostKey.PublicKey()))); err != nil {
		return err
	}
	block, err := hostBlock(hostEntry{
		Host:         t.names.Container,
		Branch:       t.branch,
		Executable:   m.Executable,
		ContainerCLI: m.ContainerCLI,
		ID:           m.Repo.ID,
		Slug:         t.names.Slug,
	})
	if err != nil {
		return err
	}
	if err := writeIfChanged(r, sshConfigFile, []byte(block)); err != nil {
		return err
	}

	o := runtime.ExecOptions{User: AgentUser, Stdin: bytes.NewReader(hostKeyPEM)}
	if _, err := m.exec(t.names.Container, o, "sh", "-c", sshKeysScript, "sh", publicKeyLine(client.PublicKey())); err != nil {
		return fmt.Errorf("writing the SSH keys in the crannog: %w", err)
	}
	return nil
}

// writeSSHInclude writes ~/.crannog/ssh/config, which includes the Host
// blocks of all crannogs. The user includes it in ~/.ssh/config.
func (m *Manager) writeSSHInclude() error {
	r, err := privateDir(m.CrannogHome.Root, home.SSH)
	if err != nil {
		return err
	}
	defer r.Close()
	return writeIfChanged(r, sshConfigFile, []byte(includeConfig))
}

// crannogHomeInSSHConfig is the crannog home in the SSH files: ssh
// expands ~ to the user's home, and the crannog home is always
// ~/.crannog (hostfs). Paths below it consist of names crannog chose and
// validated, so they need no quoting. Unquoted, zsh's completion of ssh
// hosts can follow the Include; it does not remove quotes.
const crannogHomeInSSHConfig = "~/.crannog"

// includeConfig is ~/.crannog/ssh/config.
var includeConfig = "# Written by crannog. Include it at the top of ~/.ssh/config to reach\n" +
	"# crannogs with SSH:  Include ~/.crannog/ssh/config\n" +
	"Include " + filepath.Join(crannogHomeInSSHConfig, home.CrannogPath("*", "*"), sshDir, sshConfigFile) + "\n"

// privateDir opens the directory name in parent, creating it readable by
// the user only if it is missing.
func privateDir(parent *os.Root, name string) (*os.Root, error) {
	if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	return hostfs.OpenDir(parent, name, false)
}

// ensureClientKey returns the client key in ~/.crannog/ssh, creating it if
// it is missing. Crannog processes of several repositories may create it at
// the same time: the key is linked into place, so the first one wins and
// the others use it.
func ensureClientKey(h home.CrannogHome) (ssh.Signer, error) {
	r, err := privateDir(h.Root, home.SSH)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	signer, _, err := readKey(r, clientKeyFile)
	if errors.Is(err, fs.ErrNotExist) {
		signer, err = createLinked(r, clientKeyFile, "crannog")
	}
	if err != nil {
		return nil, err
	}
	pub := publicKeyLine(signer.PublicKey()) + " crannog\n"
	if err := writeIfChanged(r, clientKeyFile+".pub", []byte(pub)); err != nil {
		return nil, err
	}
	return signer, nil
}

// createLinked creates a key under a temporary name and links it to name,
// which must not exist. If another process created name meanwhile, its key
// is returned.
func createLinked(r *os.Root, name, comment string) (ssh.Signer, error) {
	data, err := newKey(comment)
	if err != nil {
		return nil, err
	}
	tmp := name + ".crannog-tmp-" + randomName()
	if err := renameio.WriteFile(tmp, data, 0o600, renameio.WithRoot(r)); err != nil {
		return nil, err
	}
	defer r.Remove(tmp)
	if err := r.Link(tmp, name); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	signer, _, err := readKey(r, name)
	return signer, err
}

// ensureKey returns the key in name and its PEM encoding, creating it if it
// is missing. The caller holds the crannog's lock.
func ensureKey(r *os.Root, name, comment string) (ssh.Signer, []byte, error) {
	signer, data, err := readKey(r, name)
	if !errors.Is(err, fs.ErrNotExist) {
		return signer, data, err
	}
	if data, err = newKey(comment); err != nil {
		return nil, nil, err
	}
	if err := renameio.WriteFile(name, data, 0o600, renameio.WithRoot(r)); err != nil {
		return nil, nil, err
	}
	return readKey(r, name)
}

// readKey reads a private key and returns it with its PEM encoding.
func readKey(r *os.Root, name string) (ssh.Signer, []byte, error) {
	data, err := r.ReadFile(name)
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w; delete it to have crannog create a new one", filepath.Join(r.Name(), name), err)
	}
	return signer, data, nil
}

// newKey returns a new Ed25519 private key in OpenSSH's format.
func newKey(comment string) ([]byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(block), nil
}

func randomName() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// writeIfChanged writes a file readable by the user only, unless it holds
// data already.
func writeIfChanged(r *os.Root, name string, data []byte) error {
	if old, err := r.ReadFile(name); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return renameio.WriteFile(name, data, 0o600, renameio.WithRoot(r))
}

// publicKeyLine returns a public key as in authorized_keys, without a
// newline.
func publicKeyLine(key ssh.PublicKey) string {
	return strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(key)), "\n")
}

// knownHostsLine returns the known_hosts line of a crannog's host key, by
// the host key alias of its Host block.
func knownHostsLine(alias string, key ssh.PublicKey) string {
	return alias + " " + publicKeyLine(key) + "\n"
}

// hostEntry is what a crannog's Host block is made of.
type hostEntry struct {
	// Host is the name users connect to: the crannog's runtime name.
	Host   string
	Branch string
	// Executable is the crannog binary the ProxyCommand runs.
	Executable string
	// ContainerCLI is the container CLI the ProxyCommand runs.
	ContainerCLI string
	ID           string
	Slug         string
}

// hostBlock returns the Host block of a crannog. In an SSH config the
// first value of an option wins, so the block sets everything that keeps
// the user's own keys, agent and settings out of the crannog; options a
// user's Host * sets that it does not set still apply (see the docs).
func hostBlock(e hostEntry) (string, error) {
	if !repository.ValidID(e.ID) || !slug.Valid(e.Slug) {
		return "", fmt.Errorf("invalid crannog %s %s", e.ID, e.Slug)
	}
	identity := filepath.Join(crannogHomeInSSHConfig, home.SSH, clientKeyFile)
	known := filepath.Join(crannogHomeInSSHConfig, home.CrannogPath(e.ID, e.Slug), sshDir, knownHostsFile)
	for _, p := range []string{e.Executable, e.ContainerCLI} {
		if strings.ContainsFunc(p, isControl) {
			return "", fmt.Errorf("the path %q cannot be used in an SSH config", p)
		}
	}
	// ssh expands %-tokens in the ProxyCommand, then a shell runs it.
	proxy := strings.ReplaceAll(shell.Join([]string{e.Executable, "ssh-proxy", "--container-cli", e.ContainerCLI, e.ID, e.Slug}), "%", "%%")
	branch := strings.Map(func(r rune) rune {
		if isControl(r) {
			return '?'
		}
		return r
	}, e.Branch)
	return fmt.Sprintf(`# Written by crannog for the crannog of %s; rewritten by every
# crannog checkout of this branch, removed by crannog destroy.
Host %s
  HostKeyAlias %s
  User %s
  ProxyCommand %s
  IdentityFile %s
  IdentitiesOnly yes
  IdentityAgent none
  ForwardAgent no
  ForwardX11 no
  PermitLocalCommand no
  StrictHostKeyChecking yes
  UpdateHostKeys no
  UserKnownHostsFile %s
`, branch, e.Host, e.Host, AgentUser, proxy, identity, known), nil
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// sshKeysScript writes the crannog's host key, from stdin, and $1 as
// authorized_keys to sshDirInCrannog.
const sshKeysScript = `set -eu
umask 077
d=` + sshDirInCrannog + `
mkdir -p "$d"
chmod 700 "$d"
cat > "$d/` + hostKeyFile + `.crannog-tmp"
printf '%s\n' "$1" > "$d/authorized_keys.crannog-tmp"
mv -f "$d/` + hostKeyFile + `.crannog-tmp" "$d/` + hostKeyFile + `"
mv -f "$d/authorized_keys.crannog-tmp" "$d/authorized_keys"`

// sshdConfigScript prints the sshd config of a connection. sshd does not
// pass its environment on to sessions: SetEnv gives them the crannog's
// variables, the proxy's among them, except those sshd sets itself and
// values an SSH config cannot hold. It has no here-document: sh on macOS
// writes one to a file in /tmp, which Seatbelt does not allow in tests.
const sshdConfigScript = `d=` + sshDirInCrannog + `
printf '%s\n' \
  "HostKey $d/` + hostKeyFile + `" \
  "AuthorizedKeysFile $d/authorized_keys" \
  "AuthenticationMethods publickey" \
  "PasswordAuthentication no" \
  "KbdInteractiveAuthentication no" \
  "UsePAM no" \
  "StrictModes no" \
  "PidFile none" \
  "AllowAgentForwarding no" \
  "X11Forwarding no" \
  "PrintMotd no" \
  "Subsystem sftp internal-sftp"
awk 'BEGIN {
  line = ""
  for (k in ENVIRON) {
    if (k !~ /^[A-Za-z_][A-Za-z0-9_]*$/) continue
    if (k ~ /^(HOME|USER|LOGNAME|SHELL|PWD|OLDPWD|SHLVL|_|TERM|MAIL)$/ || k ~ /^SSH_/) continue
    v = ENVIRON[k]
    if (v ~ /["\\]/ || v ~ /[[:cntrl:]]/) continue
    line = line " \"" k "=" v "\""
  }
  if (line != "") print "SetEnv" line
}'`

// sshdScript serves one SSH connection on stdin and stdout with sshd -i
// as the agent user. It runs on every connection: the config takes the
// crannog's current environment.
const sshdScript = `set -eu
d=` + sshDirInCrannog + `
sshd=$(command -v sshd 2>/dev/null || true)
if [ -z "$sshd" ] && [ -x /usr/sbin/sshd ]; then sshd=/usr/sbin/sshd; fi
if [ -z "$sshd" ]; then
  echo "crannog: this crannog has no sshd; its image needs OpenSSH's server (Debian: openssh-server)" >&2
  exit 1
fi
if [ ! -s "$d/` + hostKeyFile + `" ]; then
  echo "crannog: SSH is not prepared in this crannog; crannog checkout prepares it" >&2
  exit 1
fi
umask 077
config() {
` + sshdConfigScript + `
}
config > "$d/sshd_config.$$"
mv -f "$d/sshd_config.$$" "$d/sshd_config"
exec "$sshd" -i -f "$d/sshd_config"`

// ReplaceFunc replaces the process with a command in a running container,
// see runtime.CLI.Replace.
type ReplaceFunc func(name string, o runtime.ExecOptions, args ...string) error

// SSHProxy connects the standard input and output of this process to sshd
// in the crannog with the given repository id and branch slug: the
// ProxyCommand of the crannog's Host block. Like the git causeway, it
// never starts a crannog. It only returns on errors.
func SSHProxy(h home.CrannogHome, rt runtime.Runtime, replace ReplaceFunc, id, branchSlug string) error {
	if !repository.ValidID(id) || !slug.Valid(branchSlug) {
		return fmt.Errorf("invalid crannog %s %s", id, branchSlug)
	}
	n := naming.New(id[:8], branchSlug)
	d := newHostDir(h, id, branchSlug)
	branch, err := d.Branch()
	if err != nil {
		return err
	}
	hash, err := d.Hash()
	if err != nil {
		return err
	}
	if branch == "" || hash == "" {
		return fmt.Errorf("there is no crannog %s", n.Container)
	}
	list, err := bounded(func(ctx context.Context) ([]runtime.Container, error) { return rt.List(ctx, n.Container) })
	if err != nil {
		return err
	}
	if !hasRunning(list, n.Container) {
		return fmt.Errorf("the crannog of %s%s is not running; start it with: crannog checkout -d %s",
			branch, repositoryHint(h, id), shell.Quote(branch))
	}
	return replace(n.Container, runtime.ExecOptions{User: AgentUser, Interactive: true}, "sh", "-c", sshdScript)
}

// repositoryHint names the repository of an id for messages, from the git
// common dir crannog recorded, or "".
func repositoryHint(h home.CrannogHome, id string) string {
	r, err := h.OpenDir(home.RepositoryPath(id), false)
	if err != nil {
		return ""
	}
	defer r.Close()
	data, err := r.ReadFile(home.PathFile)
	if err != nil {
		return ""
	}
	dir := strings.TrimSuffix(string(data), "\n")
	if filepath.Base(dir) == ".git" {
		dir = filepath.Dir(dir)
	}
	if strings.ContainsFunc(dir, isControl) {
		return ""
	}
	return " in " + dir
}
