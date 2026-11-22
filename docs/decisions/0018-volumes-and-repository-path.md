# 0018. Volumes and repository path

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

A crannog's container is replaced when its setup changes
([ADR 0022](0022-rebuilds.md)); the repository and the agents' state must
survive that. Agents keep state per working directory, so the repository
path inside a crannog decides which state crannogs share. Some agent state
is not safe to share between VMs ([ADR 0011](0011-agent-home.md)).

## Decision

- **Volumes:** every crannog has two named volumes. `/work` holds the
  repository and the agent's worktrees, `/state` holds agent state that must
  not be shared between crannogs. Both survive rebuilding the crannog and
  are deleted on destroy.
- **Fixed:** `/work` and `/state` are not part of `config.yml`
  ([ADR 0025](0025-configuration-schema.md)); the git causeway depends on
  `/work`.
- **Repository path:** inside the crannog, the repository lives at `/work`
  followed by the absolute path of the repository's main worktree on the
  host, e.g. `/work/Users/me/code/app`. This also applies when the crannog
  is built from another worktree.

## Alternatives

- **The same path for every repository (`/work/repo`):** agents' state per
  working directory would mix across repositories.
- **The volume at the repository's parent directory:** a repository at `/repo`
  would put a volume at `/`.
- **One state volume per repository:** named volumes cannot be mounted by two
  running crannogs, and the state on it is exactly what cannot be shared.
- **`/work` and `/state` in `config.yml`:** a changed path breaks the git
  causeway; nobody needs another path.

## Consequences

- All crannogs of a repository see the same agent sessions and memory, like
  several local sessions in one checkout.
- Paths inside a crannog differ from the host by the `/work` prefix.
- Anything outside `/work`, `/state` and the mounts, e.g. software installed
  in the container's writable layer, survives stop and start but is lost
  when the crannog is rebuilt.
