// Package instructions defines crannog's managed instructions block in each
// agent's global instruction file in the agent home (ADR 0029).
package instructions

import "strings"

// Markers of the managed instructions block.
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

// ManagedBlock is the managed instructions block, markers included.
const ManagedBlock = Begin + `
# crannog

You are working inside a crannog: an isolated sandbox with its own Linux VM
that crannog built for this repository. crannog manages this block and
replaces it whenever a session starts; edits to it have no lasting effect.

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

// Update returns content with the managed instructions block replaced by
// block, or with block appended if content has none.
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
