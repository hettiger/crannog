# destroy

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0013](../decisions/0013-host-file-access.md), [0015](../decisions/0015-git-causeway.md), [0019](../decisions/0019-sessions-and-auto-stop.md), [0021](../decisions/0021-crannog-state-on-the-host.md), [0027](../decisions/0027-cli-commands.md)

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
   temporary refs `refs/crannog/destroy/<slug>/*`, without writing
   `FETCH_HEAD`, and lists branches with commits not reachable from any ref
   except `refs/crannog/*` and `refs/remotes/crannog-<slug>/*`, and
   worktrees in the crannog with uncommitted changes.
   If that fails, says that it cannot check what would be lost. The check
   leaves existing refs and tags alone, disables pruning and automatic
   maintenance, and cleans up only its own temporary refs, even on failure
   or declined confirmation.
3. Says how many running sessions end, then asks for confirmation
   (`--yes` answers it; without a terminal and `--yes` it is an error).
   Declining stops the crannog again if no session runs. The loss report is a
   point-in-time check; when sessions run, the output explains that work created
   after the check is deleted too. Sessions continue during confirmation.
4. Deletes proxy and crannog, the network, both volumes, the remote and the
   host directory. The image stays. A host
   directory that is a symlink is an error, its target stays.

Cleanup refuses a remote with changed fetch mappings. A command waiting
on a lock removed by destroy fails with a retry hint rather than continuing
with an obsolete lock.

Cleanup is not transactional: when remote removal fails, containers and
volumes may already be deleted, while the remote and a host directory with
its lock remain. Repeating destroy does not resolve changed fetch mappings;
the remote configuration needs to be corrected first.
