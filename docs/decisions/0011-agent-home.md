# 0011. Agent logins in a shared agent home

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

Agents such as Claude Code and Codex log in with subscriptions (OAuth with
access and refresh tokens) or API keys and keep their login, settings and
history in the user's home directory. Logging in must be as easy as without
crannog, and once per agent, not once per crannog.

Keeping these tokens out of the crannog would require a token broker in the
proxy (see [ADR 0009](0009-iron-proxy-proxy-causeway.md), Alternatives).

Agents are built to use one directory in the home across all projects and
sessions; splitting it file by file would break with agent updates. Some
agents keep SQLite databases in WAL mode, which coordinate through shared
memory and are not safe to share between VMs.

## Decision

- **Agent tokens live in the crannog.** Each agent keeps its login, settings
  and history in its [agent home](../glossary.md#agent-home) under
  `~/.crannog/agents/<agent>` on the host, shared by all crannogs.
- **One mount:** `~/.crannog/agents` is bind-mounted into every crannog in
  the agent user's home. Nothing else of the user's or the crannog's home is
  shared.
- **Whole agent directories, relocated with the agents' own variables:**

  | Agent | Shared via | Per crannog in `/state` |
  | ----- | ---------- | ----------------------- |
  | Claude Code | `CLAUDE_CONFIG_DIR` (includes `.claude.json`) | – |
  | Codex | `CODEX_HOME` | SQLite databases via `CODEX_SQLITE_HOME` |
  | OpenCode | `OPENCODE_CONFIG_DIR` | database via `OPENCODE_DB` |
  | Pi | `PI_CODING_AGENT_DIR` | – |

  Exceptions exist only for state that is unsafe to share between VMs and
  only through the agents' official variables. The setup's `config.yml`
  sets these variables for every supported agent
  ([ADR 0028](0028-agents-in-the-default-image.md)), whether the image
  installs it or not.
- **Agent user:** the agent runs as the non-root user `agent` with
  passwordless `sudo`. The VM is the security boundary, so the agent can
  install packages and try changes to its environment. Running as non-root
  avoids tools that refuse or warn when run as root.
- **The host stays out:** crannog's host code creates
  `~/.crannog/agents` but never reads or writes in it
  ([ADR 0014](0014-host-stays-out-of-guest-writable-directories.md)).

The [spike of the agent home](../spikes/iron-proxy-and-agent-home/README.md)
confirmed: subscription logins of Claude Code and Codex in one crannog work
in another without logging in again, the agent user can write to the mount
without `sudo`, and files written in a crannog belong to the user on the
host. The [spike of the default image](../spikes/agents-default-image/README.md)
confirmed sharing through the variables, including logins shared between
two crannogs.

## Alternatives

- **Token broker in the proxy:** see ADR 0009.
- **Mounting a shared `~/.config`:** most agents do not use it, and it would
  also share everything else tools put there (tokens of `gh` or `glab`, git
  and editor config) and hide the image's own `~/.config`.
- **Sharing only selected files per agent:** breaks with agent updates, and
  tools that write files atomically replace a symlinked file with a regular
  one, silently ending the sharing.
- **Symlinks from the home into the agent home:** the variables are the
  agents' supported way and need no setup in the image.
- **Sharing SQLite databases without WAL:** relies on file locks working
  across VMs over the bind mount; OpenCode does not allow it.
- **Running the agent as root:** same security, but some tools refuse or warn.

## Consequences

- Log in once per agent; all crannogs use that login. Using several crannogs
  at once behaves like several local sessions of the agent.
- A compromised agent can read its agent's tokens and send them to allowed
  hosts.
- A compromised agent can change the shared agent home, e.g. add hooks that
  then run in other crannogs, with access to their repositories and injected
  credentials. This risk is accepted. The user's own agent configuration on
  the host is not affected.
- OpenCode keeps its credentials in its database, so its logins are per
  crannog.
