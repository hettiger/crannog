# 0010. Git causeway via a remote helper over `container exec`

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

The git causeway carries code into a crannog and the agent's results back
out. The repository must live inside the crannog
([ADR 0006](0006-apple-containers-runtime.md): no bind mounts for code), and
the agent must have no path to the user's repository on the host.

The user works with plain git (`fetch`, `push`, GUIs). crannog must not change
the user's SSH configuration (`~/.ssh`) by default, and connections to a
crannog must never offer the user's own SSH keys or ssh-agent. git cannot set
SSH options per remote: `core.sshCommand` applies to every remote of a
repository, including `origin`.

## Decision

- **Repository on a volume:** each crannog has a named volume with a normal
  (non-bare) repository that accepts pushes to its checked-out branch
  (`receive.denyCurrentBranch updateInstead`). The volume survives recreating
  the crannog's container, e.g. after a Dockerfile change, and is removed on
  destroy.
- **Remote helper:** crannog ships a git remote helper
  `git-remote-crannog`, which must be in `PATH`. It supports only the
  `connect` capability and pipes `git-upload-pack` and `git-receive-pack`
  through `container exec`. No SSH, port or keys are involved, and every git
  command that works with a remote works with it.
- **One remote per crannog:** crannog adds a remote named after the crannog
  (URL `crannog::<crannog>`) to the user's repository when building the
  crannog and removes it on destroy. It fetches `+refs/heads/*`; the remote's
  `HEAD` is not tracked (`followRemoteHEAD never`).
- **Start branch:** a crannog starts from one of the user's branches, which
  is checked out in the crannog. The agent may create further branches and
  worktrees; there are no rules which branches a crannog may use.
- **Initial transfer:** crannog pushes all local branches and tags of the
  user's repository. Branches arrive as `origin/*` without a URL, so the
  crannog looks like a fresh clone; only the start branch is a local branch.
  Only committed history enters a crannog: uncommitted changes and untracked
  files (e.g. `.env`) stay on the host.
- **Destroy** happens only when the user asks for it, never automatically.
  Before destroying, crannog fetches and shows what would be lost: branches
  with commits not reachable from the user's own refs, and worktrees with
  uncommitted changes. The user confirms; a flag skips the confirmation.
- **No changes to `~/.ssh`:** git and crannog's own shell access use
  `container exec`. SSH is only needed for IDEs.

A [spike](0010-git-causeway/README.md) confirmed all of the above on Apple
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

## Consequences

- crannog adds and removes remotes in the user's repository. `git remote`
  lists one remote per existing crannog.
- Fetching a remote only creates remote-tracking refs, which are deleted with
  the remote on destroy. Work must be merged or branched in the user's
  repository to survive a destroy.
- `git fetch origin` fails inside the crannog; agents need to be told.
- Git LFS objects and submodules are not transferred by the helper and need
  extra handling if required.
- Over `connect`, git uses protocol v0. This works, but ref negotiation is
  less efficient than v2 for repositories with many refs.
- Software installed in the crannog outside the volume (container's writable
  layer) survives stop and start, but not recreating the container.
