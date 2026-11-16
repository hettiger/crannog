# 0015. Git causeway via a remote helper over `container exec`

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

The git causeway carries code into a crannog and the agent's results back
out. The repository must live inside the crannog
([ADR 0008](0008-apple-containers-runtime.md): no bind mounts for code), and
the agent must have no path to the user's repository on the host.

The user works with plain git (`fetch`, `push`, GUIs). crannog must not change
the user's SSH configuration (`~/.ssh`) by default, and connections to a
crannog must never offer the user's own SSH keys or ssh-agent. git cannot set
SSH options per remote: `core.sshCommand` applies to every remote of a
repository, including `origin`.

## Decision

- **Repository on a volume:** each crannog has a normal (non-bare)
  repository on its work volume
  ([ADR 0018](0018-volumes-and-repository-path.md)) that accepts pushes to
  its checked-out branch (`receive.denyCurrentBranch updateInstead`).
- **Remote helper:** crannog ships a git remote helper
  `git-remote-crannog`, which must be in `PATH`. It supports only the
  `connect` capability and pipes `git-upload-pack` and `git-receive-pack`
  through `container exec`. No SSH, port or keys are involved, and every git
  command that works with a remote works with it.
- **A remote per running crannog:** crannog adds the remote
  `crannog-<slug>` with the URL `crannog::<slug>`
  ([branch slug](0017-one-crannog-per-start-branch.md)) when it starts a
  crannog and removes it when it stops it. The remote fetches
  `+refs/heads/*`; its `HEAD` is not tracked (`followRemoteHEAD never`). If
  a remote of that name with another URL exists, it is the user's: crannog
  refuses to start and never removes it. The helper never starts a crannog;
  a stopped crannog has no remote. crannog never sets a branch's upstream.
- **Start branch:** a crannog starts from one of the user's branches, which
  is checked out in the crannog. The agent may create further branches and
  worktrees; there are no rules which branches a crannog may use.
- **Initial transfer:** crannog pushes all local branches and tags of the
  user's repository. Branches arrive as `origin/*` without a URL, so the
  crannog looks like a fresh clone; only the start branch is a local branch.
  Only committed history enters a crannog: uncommitted changes and untracked
  files (e.g. `.env`) stay on the host.
- **Destroy** happens only when the user asks for it, never automatically.
  Before destroying, crannog fetches directly via `crannog::<slug>`,
  starting the crannog if needed, and shows what would be lost: branches
  with commits not reachable from the user's own refs, and worktrees with
  uncommitted changes. The user confirms; `--yes` answers the confirmation.
  If the crannog cannot be started or fetched, crannog says that it cannot
  check what would be lost and asks all the same.
- **No changes to `~/.ssh`:** git and crannog's own shell access use
  `container exec`. SSH is only needed for IDEs.

A [spike](../spikes/git-causeway/README.md) confirmed the helper on Apple
Containers.

## Alternatives

- **git over SSH with an entry in `~/.ssh/config`** (e.g. an `Include` of a
  file managed by crannog): changes the user's SSH configuration, and a broken
  entry affects every SSH connection. Needs keys, ports and host key handling
  just for git.
- **Plain SSH remote URL:** uses the user's SSH config, keys and ssh-agent
  and asks to trust the host key on first connect.
- **`core.sshCommand` per repository:** also applies to `origin`.
- **One remote for all crannogs** (helper dispatching refs to crannogs): needs
  rules mapping branches to crannogs and considerably more helper code.
- **The agent clones from the upstream host via the proxy:** misses the
  user's local, unpushed commits and bypasses the git causeway.
- **A remote named like the crannog or the branch:** not recognizable,
  clashes with remotes such as `origin`.
- **`crannog/<slug>` as remote name:** git supports slashes in remote names,
  but some tools assume the first path segment of a remote-tracking ref is
  the remote.
- **A remote from build to destroy:** the number of remotes grows with the
  crannogs lying around, and fetching a stopped crannog would need the
  helper to start it.
- **No `destroy` without a successful check, or a separate `--force`:** a
  broken crannog could not be destroyed, or needs another flag.

## Consequences

- `git remote` lists exactly the running crannogs. Remote-tracking refs of a
  crannog disappear when it stops and return after the next fetch once it
  runs again; work must be fetched and merged while a session runs.
- Removing a remote also removes upstream settings pointing to it.
- Work must be merged or branched in the user's repository to survive a
  destroy.
- A crannog that cannot start can still be destroyed, at the risk of losing
  unfetched work.
- `git fetch origin` fails inside the crannog; agents need to be told.
- Git LFS objects and submodules are not transferred by the helper and need
  extra handling if required.
- Over `connect`, git uses protocol v0. This works, but ref negotiation is
  less efficient than v2 for repositories with many refs.
