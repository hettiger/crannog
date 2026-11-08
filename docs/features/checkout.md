# checkout

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0010](../decisions/0010-git-causeway.md), [0011](../decisions/0011-repository-configuration.md), [0013](../decisions/0013-agents-default-image.md), [0014](../decisions/0014-one-crannog-per-start-branch.md), [0015](../decisions/0015-presets.md), [0016](../decisions/0016-cli-commands.md), [0018](../decisions/0018-rebuilds-with-running-sessions.md), [0019](../decisions/0019-configuration-schema.md), [0020](../decisions/0020-proxy-address.md), [0023](../decisions/0023-git-author-from-the-host.md), [0024](../decisions/0024-readiness-marker.md), [0025](../decisions/0025-session-locks-and-lifecycle.md), [0029](../decisions/0029-poc-lifecycle-details.md), [0030](../decisions/0030-poc-implementation-conventions.md), [0031](../decisions/0031-host-file-access.md)

## Summary

Opens a session in the crannog of a start branch, building, updating or
starting the crannog as needed.

## Usage

```sh
crannog checkout [branch] [-- command...] [--yes]
```

`branch` defaults to the checked-out branch and must be a local branch.
The command after `--` replaces `session.command`; each argument is passed
unchanged (`crannog checkout -- claude -p "say hi"`). `--yes` answers
confirmations, never setup approvals.

## Behavior

1. Repairs the repository's crannogs (stops those without a session,
   removes stale `crannog-*` remotes).
2. Reads `.crannog/`: from the working tree if the branch is checked out in
   the current worktree, otherwise from the branch's latest commit, else the
   preset `default`. Prints the source and a note if uncommitted changes in
   this worktree are not used. An invalid `config.yml` is an error, as is a
   `config.yml` or `iron-proxy.yaml` with control characters other than tab
   and line breaks, so escape sequences cannot hide lines of the approval
   diff. A `.crannog` file or symlink is
   an error; a directory deleted in the working tree counts as missing,
   even if its files are still in the index.
3. Takes the crannog's lock, waiting for other crannog commands.
4. New crannog: asks for approval of content not approved before (diff to
   the most recent approval; presets need none), builds the image if
   missing, pulls the proxy image, generates the CA, creates network, volumes, proxy and
   crannog, starts them, adds the remote
   `crannog-<slug>`, initializes the repository at `/work/<host path>` and
   pushes all branches as `origin/*`, all tags and the start branch directly
   to the crannog URL, ignoring a remote's separate `pushurl`. On
   failure, everything created is removed again.
5. Existing crannog: if `.crannog/` differs from the crannog's copy, asks
   for approval (new content) or confirmation (a rebuild that ends running
   sessions), then reloads the proxy (only `iron-proxy.yaml` changed) or
   rebuilds both containers. With running sessions and no other question,
   asks before starting another session; without a terminal it only warns.
   Starts proxy and crannog if stopped, waits for the readiness marker and
   adds the remote. A foreign remote of that name is an error.
6. Before the session: writes `crannog-proxy` to `/etc/hosts` and mirrors
   `user.name` and `user.email` of the host repository.
7. Registers the session (`sessions/<pid>`, locked), releases the lock and
   runs the command as `agent` in the agent's login shell in the
   repository, with a terminal if stdin and stdout are terminals.
8. When the session ends: if no other session runs, stops crannog and proxy
   and removes the remote. Exits with the session's exit code.

The crannog remote is temporary. Removing it also removes Git's upstream
configuration for local branches tracking that remote; their local branches
and commits remain. Starting another session recreates the remote, not those
upstream settings.

The managed instructions block is written into each agent home on every
build and rebuild. A symlinked agent home or instruction file and an
instruction file larger than 1 MiB are errors. The proxy mounts its
directory read-only. A symlinked directory in crannog's state in
`~/.crannog` is an error, never followed. The approval diff is built in a
temporary directory `crannog-diff-*` in `$TMPDIR`.

A lock removed or replaced while checkout waits is an error with a retry
hint. Remote cleanup refuses changed fetch mappings rather than deleting
unrelated refs.

The 60-second deadline for the proxy health and the readiness marker
([ADR 0024](../decisions/0024-readiness-marker.md)) also ends a probe that
hangs. It is not a timeout for the whole checkout.

If containers are missing, checkout explains that destroy deletes their work
volume. When the selected setup requires a rebuild, it also suggests
`crannog apply <branch>` with that setup, which recreates the containers
while preserving the volumes.
