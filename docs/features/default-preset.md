# default preset

- **Status:** In progress (PoC, content under review)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0009](../decisions/0009-iron-proxy-proxy-causeway.md), [0011](../decisions/0011-agent-home.md), [0012](../decisions/0012-shared-package-caches.md), [0020](../decisions/0020-readiness-marker.md), [0026](../decisions/0026-presets.md), [0028](../decisions/0028-agents-in-the-default-image.md), [0029](../decisions/0029-managed-instructions-block.md)

## Summary

The bundled preset crannog uses for repositories without `.crannog/`
(`internal/preset/bundled/default/`).

## Usage

Used automatically. `crannog preset publish default` writes it to
`.crannog/`; a user preset `~/.crannog/presets/default/` replaces it.

## Behavior

- **`config.yml`:** session `claude --dangerously-skip-permissions`, 4
  CPUs, 4G, iron-proxy 0.54.0 pinned by digest; proxy variables via
  `CRANNOG_PROXY_URL`, `NODE_USE_ENV_PROXY`, the CA for Node and Codex, the
  agent homes of Claude Code, Codex, OpenCode and Pi, Codex's and
  OpenCode's state in `/state`, npm and Playwright caches; mounts
  of the CA, the agent homes and the caches. It reads nothing from the
  host's environment.
- **`iron-proxy.yaml`:** tunnel listener `:8080`, TLS interception with the
  crannog's CA, metrics and management API on loopback, deny by default.
  Allowlist: `*.anthropic.com`, `*.claude.ai`, `*.claude.com`,
  `*.openai.com`, `*.chatgpt.com`, `opencode.ai`, `models.opencode.ai`,
  `pi.dev`, `registry.npmjs.org`, `deb.debian.org` (also serves Debian's
  security updates).
- **`container/`:** Debian trixie with the four agents and Node.js 24.
  `/etc/sudoers.d/proxy` keeps the proxy variables for `sudo`; apt sources
  stay `http://`. The entrypoint trusts the CA, owns the volumes, creates
  `/state/codex` and `/state/opencode` (they must match `CODEX_SQLITE_HOME`
  and `OPENCODE_DB` in `config.yml`), writes the readiness marker and idles
  with a TERM trap, so `container stop` does not wait for its timeout.
