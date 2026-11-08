# Spike: git causeway via a remote helper

Evidence for [ADR 0015](../../decisions/0015-git-causeway.md). Rerun it after updating
Apple Containers.

`git-remote-crannog` is a minimal remote helper written in bash for the
spike: it supports only `connect` and pipes `git-upload-pack` and
`git-receive-pack` through `container exec`.

## What it checks

1. **Images and crannog:** a small Debian image built in two versions, one
   crannog with a named volume at `/work`, an empty repository on the start
   branch.
2. **Initial transfer:** all branches as `origin/*` and all tags, the start
   branch checked out; untracked and uncommitted files stay on the host.
3. **Round trip:** the agent commits on the start branch and on a new branch
   in a worktree; fetching returns exactly these two branches; pushing into a
   dirty working tree is rejected with a failing exit code; a 20 MB binary
   arrives unchanged in both directions.
4. **Persistence:** the repository and worktree survive stop and start, and
   deleting the container and running it again with a new image on the same
   volume.
5. **Destroy summary:** branches with commits not reachable from the user's
   own refs and dirty worktrees are detected; removing the remote removes its
   remote-tracking refs, merged work stays.

## Running

Requires Apple Silicon, macOS 26, the `container` CLI and internet access for
the image build. The host's git runs with an isolated config.

```bash
./spike.sh          # automated checks, cleans up afterwards
./spike.sh --keep   # keeps the work dir for inspection
```

## Result

2026-10-31, `container` 1.6.1, Apple Git 2.50.1: **47 passed, 0 failed.**

- The helper receives no `GIT_PROTOCOL`; git uses protocol v0 over `connect`.
- 20 MB binary: about 26 MB/s for push and fetch.
- The container's writable layer survives stop and start, but not recreating
  the container.
