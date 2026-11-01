# destroy

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0010](../decisions/0010-git-causeway.md), [0014](../decisions/0014-one-crannog-per-start-branch.md), [0025](../decisions/0025-session-locks-and-lifecycle.md), [0029](../decisions/0029-poc-lifecycle-details.md), [0030](../decisions/0030-poc-implementation-conventions.md)

## Summary

Destroys the crannog of a start branch after showing what would be lost.

## Usage

```sh
crannog destroy [branch] [--yes]
```

`branch` defaults to the checked-out branch; it may name a branch that no
longer exists.

## Behavior

1. Takes the crannog's lock and starts the crannog if it is stopped.
2. Fetches the crannog's branches directly via `crannog::<slug>` into
   temporary refs and lists branches with commits not reachable from the
   user's own refs, and worktrees in the crannog with uncommitted changes.
   If that fails, says that it cannot check what would be lost.
3. Says how many running sessions end, then asks for confirmation
   (`--yes` answers it; without a terminal and `--yes` it is an error).
   Declining stops the crannog again if no session runs.
4. Deletes proxy and crannog, the network, both volumes, the remote, the
   temporary refs and the host directory. The image stays.
