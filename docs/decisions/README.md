# Decisions

Architecture Decision Records, one decision topic each, always in their
current state ([ADR 0004](0004-knowledge-base.md)). New ADRs use
[template.md](template.md). Behavior and formats are in the
[feature docs](../features/README.md).

## Project

| ADR | Decides |
| --- | ------- |
| [0001](0001-go-and-cobra.md) Go and Cobra | Go and Cobra for the CLI |
| [0002](0002-mit-license.md) MIT license | License, no per-file headers |
| [0003](0003-english-only.md) English-only | Everything in the repository is English |
| [0004](0004-knowledge-base.md) Knowledge base | `docs/` as knowledge base; what ADRs hold; revised in place until the first release |
| [0005](0005-domain-language.md) Domain language | Crannog and causeway as the only metaphors |
| [0006](0006-repository-layout.md) Repository layout | One directory per binary, Cobra commands in `crannog/cmd/`, one module, version injection, `e2e/` |
| [0007](0007-go-dependencies.md) Go dependencies | Direct dependencies and what uses the standard library |

## Sandbox

| ADR | Decides |
| --- | ------- |
| [0008](0008-apple-containers-runtime.md) Apple Containers | A VM per crannog via Apple Containers; egress only through the proxy on an internal network |
| [0009](0009-iron-proxy-proxy-causeway.md) iron-proxy | iron-proxy as proxy causeway: allowlist, TLS interception with a host-generated CA, credential injection, management API key, resources |
| [0010](0010-proxy-address.md) Proxy address | `crannog-proxy` in `/etc/hosts`, written before every session |
| [0011](0011-agent-home.md) Agent home | Agent logins and settings shared via `~/.crannog/agents`, relocated with the agents' variables; per-crannog exceptions in `/state`; the `agent` user |
| [0012](0012-shared-package-caches.md) Package caches | Package manager caches shared via `~/.crannog/cache` |
| [0013](0013-host-file-access.md) Host file access | All host file access through `os.Root` on three anchors; symlinks in crannog's state are errors |
| [0014](0014-host-stays-out-of-guest-writable-directories.md) Guest-writable directories | Host code never touches directories a guest can write to; the crannog writes there itself |

## Git causeway

| ADR | Decides |
| --- | ------- |
| [0015](0015-git-causeway.md) Git causeway | Remote helper over `container exec`, a `crannog-<slug>` remote while the crannog runs, initial transfer, the check before destroy |
| [0016](0016-git-author-from-the-host.md) Git author | `user.name` and `user.email` mirrored from the host before every session |

## Crannogs

| ADR | Decides |
| --- | ------- |
| [0017](0017-one-crannog-per-start-branch.md) One crannog per start branch | Crannogs identified by start branch; branch slugs; runtime names |
| [0018](0018-volumes-and-repository-path.md) Volumes and repository path | `/work` and `/state` volumes; the repository at `/work/<host path>` |
| [0019](0019-sessions-and-auto-stop.md) Sessions and auto-stop | Several sessions per crannog, counted with `flock` on the host; stop after the last one; self-repair |
| [0020](0020-readiness-marker.md) Readiness marker | Entrypoint writes the boot id to `/tmp/ready`; crannog waits up to 60 seconds |
| [0021](0021-crannog-state-on-the-host.md) Crannog state on the host | Layout of `~/.crannog`, repository id, when a crannog exists, cleanup of failed builds |
| [0022](0022-rebuilds.md) Rebuilds | What a rebuild replaces, proxy-only reloads, `apply` never starts, confirmation with running sessions |

## Configuration

| ADR | Decides |
| --- | ------- |
| [0023](0023-repository-configuration.md) Repository configuration | `.crannog/` per repository, native `iron-proxy.yaml`, build context, which source is read |
| [0024](0024-setup-approval.md) Setup approval | Diff and approval before a setup is used; approved once, approved always; never skipped by `--yes` |
| [0025](0025-configuration-schema.md) Configuration schema | `config.yml` keys, `CRANNOG_*` variables, interpolation, setup hash, image name |
| [0026](0026-presets.md) Presets | Bundled and user presets, `default` as fallback, no approval for presets |

## CLI

| ADR | Decides |
| --- | ------- |
| [0027](0027-cli-commands.md) CLI commands | Command set, `[branch]`, arguments after `--`, interaction and `--yes` |

## Agents

| ADR | Decides |
| --- | ------- |
| [0028](0028-agents-in-the-default-image.md) Agents in the default image | Supported agents, the default image, CA trust, permission prompts, system packages, default allowlist |
| [0029](0029-managed-instructions-block.md) Managed instructions block | A marked block in each agent's instruction file, written by the crannog before every session |
