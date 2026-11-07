# default preset

- **Status:** In progress (PoC, content under review)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0013](../decisions/0013-agents-default-image.md), [0015](../decisions/0015-presets.md), [0019](../decisions/0019-configuration-schema.md), [0020](../decisions/0020-proxy-address.md), [0022](../decisions/0022-default-image-packages-and-state.md), [0024](../decisions/0024-readiness-marker.md), [0030](../decisions/0030-poc-implementation-conventions.md)

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
  crannog's CA, metrics and management API on loopback, the allowlist of
  ADR 0013 and ADR 0022, deny by default.
- **`container/`:** Debian trixie with the four agents, Node.js 24, sudo
  keeping the proxy variables, and an entrypoint that trusts the CA, owns
  the volumes, creates `/state/codex` and `/state/opencode` and writes the
  readiness marker.
