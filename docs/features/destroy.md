# destroy

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0010](../decisions/0010-git-causeway.md), [0014](../decisions/0014-one-crannog-per-start-branch.md), [0025](../decisions/0025-session-locks-and-lifecycle.md), [0029](../decisions/0029-poc-lifecycle-details.md), [0030](../decisions/0030-poc-implementation-conventions.md), [0032](../decisions/0032-anchored-host-file-access.md)

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
   fresh temporary refs and lists branches with commits not reachable from the
   user's own refs, and worktrees in the crannog with uncommitted changes.
   If that fails, says that it cannot check what would be lost. The check
   leaves existing refs and tags alone, disables pruning and automatic
   maintenance, and cleans up only its own temporary refs, even on failure
   or declined confirmation.
3. Says how many running sessions end, then asks for confirmation
   (`--yes` answers it; without a terminal and `--yes` it is an error).
   Declining stops the crannog again if no session runs. The loss report is a
   point-in-time check; when sessions run, the output explains that work created
   after the check is deleted too. Sessions continue during confirmation
   ([ADR 0034](../decisions/0034-bounded-readiness-and-recovery-guidance.md)).
4. Deletes proxy and crannog, the network, both volumes, the remote and the
   host directory. The image stays. A host
   directory that is a symlink is an error, its target stays.

Cleanup refuses a remote with changed fetch mappings. A command waiting
on a lock removed by destroy fails with a retry hint rather than continuing
with an obsolete lock ([ADR 0033](../decisions/0033-conservative-cleanup-and-replacement.md)).

Cleanup is not transactional: when remote removal fails, containers and
volumes may already be deleted, while the remote and a host directory with
its lock remain. Repeating destroy does not resolve changed fetch mappings;
the remote configuration needs to be corrected first.
