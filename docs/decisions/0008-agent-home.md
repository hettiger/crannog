# 0008. Agent logins in a shared agent home

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

Agents such as Claude Code and Codex log in with subscriptions (OAuth with
access and refresh tokens) or API keys and keep their login, settings and
history in the user's home directory. Logging in must be as easy as without
crannog, and once per agent, not once per crannog.

Keeping these tokens out of the crannog would require a token broker in the
proxy (see [ADR 0007](0007-iron-proxy-proxy-causeway.md), Alternatives).

## Decision

- **Agent tokens live in the crannog.** Each agent keeps its login, settings
  and history in its [agent home](../glossary.md#agent-home) under
  `~/.crannog/agents/<agent>` on the host, shared by all crannogs.
- **One mount:** `~/.crannog/agents` is bind-mounted into every crannog at the
  same path in the agent user's home. The image links the locations the agent
  expects into it, e.g. `~/.claude` and `~/.claude.json` for Claude Code,
  `~/.codex` for Codex. What is shared is defined per agent; nothing else of
  the user's or the crannog's home is shared.
- **Agent user:** the agent runs as a non-root user with passwordless `sudo`.
  The VM is the security boundary, so the agent can install packages and try
  changes to its environment. Running as non-root avoids tools that refuse or
  warn when run as root.
- **No symlink following:** a crannog can create symlinks in the agent home
  that point to host paths. crannog never follows symlinks when it reads,
  copies or deletes anything in the agent home.

A [spike](0007-iron-proxy-proxy-causeway/README.md) confirmed: subscription
logins of Claude Code and Codex in one crannog work in another without logging
in again, the agent user can write to the mount without `sudo`, and files
written in a crannog belong to the user on the host.

## Alternatives

- **Token broker in the proxy:** see ADR 0007.
- **Mounting a shared `~/.config`:** most agents do not use it, and it would
  also share everything else tools put there (tokens of `gh` or `glab`, git
  and editor config) and hide the image's own `~/.config`.
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
- Agent state that is not safe to share across VMs (e.g. SQLite databases in
  WAL mode) must stay out of the shared part of the agent home.
