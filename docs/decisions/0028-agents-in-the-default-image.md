# 0028. Agents in the default image

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog supports Claude Code, Codex, OpenCode and Pi from the start. One
agent must be able to call another, e.g. Claude Code asks OpenCode for a
review, so several agents run in one crannog. Agents and the tools they
start need the network through the proxy and must trust the crannog's CA.

## Decision

- **Supported agents:** crannog knows Claude Code (`claude`, `CLAUDE.md`),
  Codex (`codex`, `AGENTS.md`), OpenCode (`opencode`, `AGENTS.md`) and Pi
  (`pi`, `AGENTS.md`): their agent homes
  ([ADR 0011](0011-agent-home.md)) and instruction files
  ([ADR 0029](0029-managed-instructions-block.md)).
- **Default image:** the `default` preset's Dockerfile installs all four
  agents, one block per agent that can be removed on its own. It is based on
  Debian with zsh, Node.js (required by Pi), ripgrep, fd and bubblewrap.
  Agents are installed with their official installers as the agent user, so
  they can update themselves; auto-updates stay enabled.
- **Environment in `config.yml`:** the agent home variables, the proxy
  variables, `NODE_EXTRA_CA_CERTS`, `CODEX_CA_CERTIFICATE` and
  `NODE_USE_ENV_PROXY=1` are part of the default `config.yml`.
- **CA:** the crannog's certificate is mounted at `/etc/crannog/ca.crt` and
  added to the system trust store when the crannog starts.
- **Session:** the default `session.command` starts Claude Code. Agents run
  without permission prompts by default (e.g.
  `--dangerously-skip-permissions`): the crannog is the security boundary.
- **OpenCode logins:** ChatGPT works with the "Codex device code" method;
  Anthropic models work through Claude Code's shared login.
- **System packages:** `sudo` keeps the proxy variables
  (`Defaults env_keep`), so `sudo apt-get` works through the proxy. apt
  sources stay as Debian ships them, with `http://`: the proxy applies the
  allowlist to plain HTTP, and apt verifies package signatures.
- **Default allowlist:** the agents' hosts, `registry.npmjs.org` and
  `deb.debian.org`. GitHub and telemetry hosts are not included.

The [spike of the default image](../spikes/agents-default-image/README.md)
confirmed this on Apple Containers, including Claude Code calling OpenCode;
a [later spike](../spikes/default-image-and-network/README.md) confirmed
`sudo apt-get` through the proxy.

## Alternatives

- **One agent per crannog:** agents could not call each other.
- **Disabling auto-updates:** agents would only update when the image is
  rebuilt.
- **Switching apt sources to `https://`:** works too, but changes the image
  without adding security.

## Consequences

- Agents behave like local use: one login, settings and sessions shared by
  all crannogs, per repository through the repository path
  ([ADR 0018](0018-volumes-and-repository-path.md)).
- Adding an agent needs a change to crannog, not only to the preset.
- An OpenCode login with ChatGPT is needed in each crannog, through a method
  OpenCode labels as legacy.
- OpenCode also writes snapshots and logs to `~/.local/share/opencode` and
  `~/.local/state/opencode`, and agent self-updates and system packages land
  outside the volumes; they are lost when the crannog is rebuilt.
- Agents need the allowlisted hosts; whatever they reach beyond is denied and
  shows up in the proxy's log.
