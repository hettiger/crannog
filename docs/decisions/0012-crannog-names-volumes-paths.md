# 0012. Crannog names, volumes and paths

- **Status:** Accepted, partially superseded by [0014](0014-one-crannog-per-start-branch.md) (names, selection)
- **Date:** 2026-11-01

## Context

A repository can have several crannogs, also several started from the same
branch, because the agent may create its own branches
([ADR 0010](0010-git-causeway.md)). The user works in the context of a
repository and a branch and needs to address crannogs without long names.

ADR 0010 names the git remote after the crannog. A remote named like the
crannog does not show that it is one, and it can clash with the user's own
remotes such as `origin`.

Names of containers, volumes and networks are global in Apple Containers.
Agents keep state per working directory, so the repository path inside a
crannog decides which state crannogs share.

## Decision

- **Name:** each crannog has a name that is unique within its repository,
  lowercase kebab-case (`^[a-z0-9]+(-[a-z0-9]+)*$`), validated on creation. By
  default it is derived from the start branch: every character outside
  `a-z0-9` becomes `-`, repeated dashes are collapsed and leading or trailing
  ones removed (`feature/Foo_bar` → `feature-foo-bar`). A name can always be
  given explicitly.
- **Start branch:** by default the checked-out branch; another branch can be
  given and must be given on a detached HEAD. crannog stores it per crannog.
- **Conflicts:** if the name is taken by a crannog or its remote name by any
  existing remote, crannog refuses and asks for another name. It never picks
  a name on its own.
- **Selection:** commands take a name. Without one, crannog uses the crannogs
  whose start branch is checked out: exactly one is used, none is an error
  listing the repository's crannogs, several are offered for selection in a
  terminal and are an error otherwise.
- **Remote:** the git remote is named `crannog-<name>`, its URL is
  `crannog::<name>`. This supersedes the remote naming of ADR 0010. A branch
  `feature/x` fetched from crannog `feature-x` appears as
  `crannog-feature-x/feature/x`.
- **Runtime names:** crannog derives global names from the first eight
  characters of the repository's `<id>` ([ADR 0011](0011-repository-configuration.md)):

  ```
  crannog-<id8>-<name>          # agent container
  crannog-<id8>-<name>-proxy    # proxy container
  crannog-<id8>-<name>-net      # internal network
  crannog-<id8>-<name>-work     # volume at /work
  crannog-<id8>-<name>-state    # volume at /state
  ```

- **Volumes:** `/work` holds the repository and the agent's worktrees,
  `/state` holds agent state that must not be shared between crannogs. Both
  survive rebuilding the crannog and are deleted on destroy.
- **Repository path:** inside the crannog, the repository lives at `/work`
  followed by the absolute path of the repository's main worktree on the host,
  e.g. `/work/Users/me/code/app`. This also applies when the crannog is built
  from another worktree.

## Alternatives

- **Remote named like the crannog:** not recognizable, clashes with remotes
  such as `origin`.
- **`crannog/<name>`:** git supports slashes in remote names, but some tools
  assume the first path segment of a remote-tracking ref is the remote.
- **Selecting crannogs by branch only:** several crannogs can share a start
  branch.
- **Appending a number on name conflicts:** creates crannogs the user does not
  know about.
- **The same path for every repository (`/work/repo`):** agents' state per
  working directory would mix across repositories.
- **The volume at the repository's parent directory:** a repository at `/repo`
  would put a volume at `/`.
- **One state volume per repository:** named volumes cannot be mounted by two
  running crannogs, and the state on it is exactly what cannot be shared.

## Consequences

- Crannogs of different repositories never collide; within a repository, the
  user addresses them by short names or not at all.
- All crannogs of a repository see the same agent sessions and memory, like
  several local sessions in one checkout.
- Paths inside a crannog differ from the host by the `/work` prefix.
- Anything outside `/work`, `/state` and the mounts is lost when the crannog is
  rebuilt.
