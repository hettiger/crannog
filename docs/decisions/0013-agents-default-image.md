# 0013. Agents in the default image

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog supports Claude Code, Codex, OpenCode and Pi from the start. One
agent must be able to call another, e.g. Claude Code asks OpenCode for a
review, so several agents run in one crannog.

[ADR 0008](0008-agent-home.md) shares each agent's login and settings through
the agent home and links the agents' expected locations into it. Agents are
built to use one directory in the home across all projects and sessions;
splitting it file by file would break with agent updates. Some agents keep
SQLite databases in WAL mode, which coordinate through shared memory and are
not safe to share between VMs.

## Decision

- **Default image:** crannog's default `.crannog/container/Dockerfile`
  ([ADR 0011](0011-repository-configuration.md)) installs all four agents,
  one block per agent that can be removed on its own. It is based on Debian
  with zsh, Node.js (required by Pi), ripgrep, fd and bubblewrap. Agents are
  installed with their official installers as the agent user, so they can
  update themselves; auto-updates stay enabled.
- **Whole agent directories are shared:** each agent's directory lives in the
  agent home and is relocated there with the agent's own environment
  variables instead of symlinks. This supersedes the symlinks of ADR 0008.

  | Agent | Shared via | Per crannog in `/state` |
  | ----- | ---------- | ----------------------- |
  | Claude Code | `CLAUDE_CONFIG_DIR` (includes `.claude.json`) | – |
  | Codex | `CODEX_HOME` | SQLite databases via `CODEX_SQLITE_HOME` |
  | OpenCode | `OPENCODE_CONFIG_DIR` | database via `OPENCODE_DB` |
  | Pi | `PI_CODING_AGENT_DIR` | – |

  Exceptions exist only for state that is unsafe to share between VMs and
  only through the agents' official variables.
- **Environment in `config.yml`:** these variables, the proxy variables,
  `NODE_EXTRA_CA_CERTS`, `CODEX_CA_CERTIFICATE` and `NODE_USE_ENV_PROXY=1` are
  part of the default `config.yml`. crannog sets them for every supported
  agent, whether the image installs it or not.
- **CA:** the crannog's certificate is mounted at `/etc/crannog/ca.crt` and
  added to the system trust store when the crannog starts.
- **OpenCode logins:** OpenCode keeps its credentials in its database, so its
  logins are per crannog. ChatGPT works with the "Codex device code" method;
  Anthropic models work through Claude Code's shared login.
- **Session:** `session.command` in `config.yml` sets what a session starts:
  an agent or a shell. It runs as the agent user in the repository as a login
  shell. The default is Claude Code. Agents run without permission prompts by
  default (e.g. `--dangerously-skip-permissions`): the crannog is the security
  boundary.
- **Instructions:** crannog keeps a marked block
  (`<!-- crannog:begin -->` … `<!-- crannog:end -->`) in each agent's global
  instruction file in the agent home (`CLAUDE.md`, `AGENTS.md`). It explains
  the crannog: commit to return results, `origin` has no URL, network only via
  the proxy's allowlist, `sudo` is available. crannog replaces the block on
  every build to match its version, adds it if missing, and leaves the rest of
  the file alone. It writes atomically and refuses to write through a
  symlink.
- **Default allowlist:** `*.anthropic.com`, `*.claude.ai`, `*.claude.com`,
  `*.openai.com`, `*.chatgpt.com`, `opencode.ai`, `models.opencode.ai`,
  `pi.dev`, `registry.npmjs.org`. GitHub and telemetry hosts are not included.

A [spike](0013-agents-default-image/README.md) confirmed this on Apple
Containers, including logins shared between two crannogs and Claude Code
calling OpenCode.

## Alternatives

- **One agent per crannog:** agents could not call each other.
- **Sharing only selected files per agent:** breaks with agent updates, and
  tools that write files atomically replace a symlinked file with a regular
  one, silently ending the sharing.
- **Sharing SQLite databases without WAL:** relies on file locks working
  across VMs over the bind mount; OpenCode does not allow it.
- **Symlinks from the home into the agent home:** the variables are the
  agents' supported way and need no setup in the image.
- **Disabling auto-updates:** agents would only update when the image is
  rebuilt.
- **Instructions written once without updates:** become outdated when crannog
  changes.

## Consequences

- Agents behave like local use: one login, settings and sessions shared by all
  crannogs, per repository through the repository path
  ([ADR 0012](0012-crannog-names-volumes-paths.md)).
- An OpenCode login with ChatGPT is needed in each crannog, through a method
  OpenCode labels as legacy.
- OpenCode also writes snapshots and logs to `~/.local/share/opencode` and
  `~/.local/state/opencode`, and agent self-updates land outside the volumes;
  both are lost when the crannog is rebuilt.
- Agents need the allowlisted hosts; whatever they reach beyond is denied and
  shows up in the proxy's log.
- Changing the content of the managed block by hand has no lasting effect.
