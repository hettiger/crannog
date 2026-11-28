# destroy

- **Status:** Implemented
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0013](../decisions/0013-host-file-access.md), [0015](../decisions/0015-git-causeway.md), [0019](../decisions/0019-sessions-and-auto-stop.md), [0021](../decisions/0021-crannog-state-on-the-host.md), [0027](../decisions/0027-cli-commands.md)

## Summary

Destroys the crannog of a start branch after a confirmation.

## Usage

```sh
crannog destroy [branch] [--yes]
```

`branch` defaults to the checked-out branch; it may name a branch that no
longer exists. It completes to the start branches of the repository's
crannogs.

## Behavior

1. Takes the crannog's lock. If no session runs and nothing can be lost,
   continues with step 4 without asking: the remains of an interrupted
   first build, or a crannog whose work volume is missing
   ([ADR 0015](../decisions/0015-git-causeway.md)).
2. Says that commits that are not in the user's repository and
   uncommitted changes in the crannog are lost, and how many running
   sessions end, then asks for confirmation (`--yes` answers it; without
   a terminal and `--yes` it is an error). It does not look into the
   crannog and starts, fetches and recreates nothing. Sessions continue
   during confirmation; declining changes nothing.
3. Ends the running sessions: sends SIGTERM to every process of the
   `agent` user in the crannog, except those in the process session of the
   crannog's main process, and waits until no session runs, without a
   timeout, with a spinner in a terminal. Their programs restore the
   terminals themselves. Processes that ignore SIGTERM, as interactive
   shells do, get SIGHUP as soon as they have no child process. Ctrl-C ends
   destroy with nothing deleted; the processes in the crannog were asked to
   end all the same.
4. Deletes proxy and crannog, the network, both volumes, the remote and the
   host directory; containers, a network or volumes that are missing are
   skipped. The image stays. A host
   directory that is a symlink is an error, its target stays.

Cleanup refuses a remote with changed fetch mappings. A command waiting
on a lock removed by destroy fails with a retry hint rather than continuing
with an obsolete lock. A session that outlives its crannog says that the crannog
was destroyed ([checkout](checkout.md)).

Cleanup is not transactional: when remote removal fails, containers and
volumes may already be deleted, while the remote and a host directory with
its lock remain. Repeating destroy does not resolve changed fetch mappings;
the remote configuration needs to be corrected first.
