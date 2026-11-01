// Package instructions keeps crannog's managed block in each agent's global
// instruction file in the agent home (ADR 0013), and creates the agent
// homes.
package instructions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
)

// Markers of the managed block.
const (
	Begin = "<!-- crannog:begin -->"
	End   = "<!-- crannog:end -->"
)

// Agent is a supported agent and its global instruction file, relative to
// its agent home.
type Agent struct {
	Name string
	File string
}

// Agents are the supported agents. Their agent homes are
// ~/.crannog/agents/<name>, relocated there by config.yml.
var Agents = []Agent{
	{Name: "claude", File: "CLAUDE.md"},
	{Name: "codex", File: "AGENTS.md"},
	{Name: "opencode", File: "AGENTS.md"},
	{Name: "pi", File: "AGENTS.md"},
}

// Block is the managed block, markers included.
const Block = Begin + `
# crannog

You are working inside a crannog: an isolated sandbox with its own Linux VM
that crannog built for this repository. crannog manages this block and
replaces it on every build; edits to it have no lasting effect.

- **Return results as commits.** The user fetches this repository's
  branches with git and decides what to keep. Uncommitted changes and files
  outside the repository never reach the user. You may create branches and
  worktrees.
- **` + "`origin`" + ` has no URL.** The repository arrived like a fresh clone:
  other branches are ` + "`origin/*`" + ` refs, but fetching, pulling or pushing
  ` + "`origin`" + ` fails. The user may push new commits into this repository.
- **Network only through the proxy.** All traffic goes through an HTTP(S)
  proxy that allows only listed domains; other requests fail, usually with
  403. If you need a domain, ask the user to add it. The proxy intercepts TLS
  with this crannog's CA, which the system trust store and Node trust.
- **` + "`sudo`" + ` works without a password,** e.g. ` + "`sudo apt-get install`" + `.
  Only the repository (` + "`/work`" + `), ` + "`/state`" + ` and the shared agent and cache
  directories survive a rebuild of the crannog; installed packages do not.
- **The crannog runs while a session does.** When the last session ends,
  the crannog stops and background processes end with it.
- **Other agents** may be installed (` + "`claude`, `codex`, `opencode`, `pi`" + `);
  you can call them, e.g. to ask for a review.
- **The crannog's setup is ` + "`.crannog/`" + `** in the repository; the user may
  ask you to adapt it to the project. ` + "`config.yml`" + ` sets the session
  command, resources, proxy image, environment and mounts;
  ` + "`iron-proxy.yaml`" + ` configures the proxy (e.g. its allowlist);
  ` + "`container/`" + ` is the image's build context. Keep what crannog relies on:
  the proxy variables, CA and mounts in ` + "`config.yml`" + `, the listeners and CA
  paths in ` + "`iron-proxy.yaml`" + `, and the entrypoint writing the boot id
  (` + "`/proc/sys/kernel/random/boot_id`" + `) to ` + "`/tmp/ready`" + ` once the crannog
  is prepared. Changes take effect only after the user fetched and approved
  them on the host.
` + End

// Update returns content with the managed block replaced by block, or with
// block appended if content has none.
func Update(content, block string) string {
	start := strings.Index(content, Begin)
	if start >= 0 {
		if rel := strings.Index(content[start:], End); rel >= 0 {
			end := start + rel + len(End)
			return content[:start] + block + content[end:]
		}
	}
	// Separate the block from existing content by one empty line.
	switch {
	case content == "", strings.HasSuffix(content, "\n\n"):
	case strings.HasSuffix(content, "\n"):
		content += "\n"
	default:
		content += "\n\n"
	}
	return content + block + "\n"
}

// EnsureHomes creates the agents directory and one agent home per agent.
// Symlinks are refused: a crannog can create symlinks in the agent home
// that point to host paths (ADR 0008).
func EnsureHomes(agentsDir string) error {
	if err := ensureDir(agentsDir); err != nil {
		return err
	}
	for _, a := range Agents {
		if err := ensureDir(filepath.Join(agentsDir, a.Name)); err != nil {
			return err
		}
	}
	return nil
}

// Write updates the managed block in each agent's instruction file. It
// writes atomically and never through a symlink.
func Write(agentsDir string) error {
	if err := EnsureHomes(agentsDir); err != nil {
		return err
	}
	for _, a := range Agents {
		p := filepath.Join(agentsDir, a.Name, a.File)
		info, err := os.Lstat(p)
		var content []byte
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return err
		case !info.Mode().IsRegular():
			return fmt.Errorf("refusing to write %s: not a regular file", p)
		default:
			content, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		}
		updated := Update(string(content), Block)
		if updated == string(content) {
			continue
		}
		mode := os.FileMode(0o644)
		if info != nil {
			mode = info.Mode().Perm()
		}
		if err := renameio.WriteFile(p, []byte(updated), mode); err != nil {
			return err
		}
	}
	return nil
}

// ensureDir creates dir if it is missing and checks that it is a directory
// and not a symlink.
func ensureDir(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}
