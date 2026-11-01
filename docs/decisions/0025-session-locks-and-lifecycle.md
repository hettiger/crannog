# 0025. Session locks and lifecycle details

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0014](0014-one-crannog-per-start-branch.md) stops a crannog when its
last session ends, removes stale remotes and stops crannogs without a session
(self-repair), and locks starting crannogs with `flock`. It does not say how
crannog knows how many sessions run, which `crannog list` shows
([ADR 0016](0016-cli-commands.md)). Counting inside the crannog would need an
environment variable crannog adds to each session, which
[ADR 0019](0019-configuration-schema.md) rules out, and processes a session
starts inherit it.

Some cases of rebuilds, `apply`, `destroy` and the proxy's resources are also
open.

## Decision

- **Host state:** the crannog's host directory gets a `sessions/` directory:

  ```
  ~/.crannog/repositories/<id>/crannogs/<slug>/
    lock          # held while a crannog is started or stopped
    sessions/
      <pid>       # one file per running session, locked by it
  ```

- **Starting a session:** crannog takes an exclusive `flock` on `lock`,
  starts the crannog if needed, creates `sessions/<pid>` with its own process
  id and takes an exclusive `flock` on it. Then it releases `lock` and runs
  the session with `container exec`, which inherits the file descriptor. If
  crannog is killed, the lock lasts as long as the session runs.
- **Counting sessions:** crannog tries to lock each file in `sessions/`
  without waiting. A file it cannot lock is a running session. A file it can
  lock belongs to an ended session and is deleted.
- **Ending a session:** crannog deletes its file, takes `lock` and counts the
  sessions. If none is left, it stops the crannog and its proxy and removes
  the remote. Start and stop both hold `lock`, so they cannot race.
- **Self-repair** ([ADR 0014](0014-one-crannog-per-start-branch.md)) stops
  running crannogs with no session whose `lock` nobody holds. It removes only
  remotes `crannog-*` whose URL is `crannog::<slug>`; a remote with another
  URL is the user's and is never removed.
- **Rebuilds** build the new image first, then replace the container. A
  failing build leaves the old container in place, so `destroy` can still
  fetch from it.
- **`apply` without a crannog** for the branch is an error that points to
  `crannog checkout`.
- **`destroy` with running sessions:** the confirmation of `destroy`
  ([ADR 0010](0010-git-causeway.md)) says that the sessions end. `--yes`
  skips it.
- **Proxy resources:** crannog starts the proxy with `--memory 256M` and the
  default number of CPUs. They are not configurable.

A [spike](0022-default-image-packages-and-state/README.md) measured the
proxy under parallel TLS downloads and an `npm install`: peak RSS 40 MB,
66 MiB of 256 MiB in `container stats`.

## Alternatives

- **One shared lock for all sessions:** shows whether sessions run, not how
  many.
- **An environment variable per session, counted in `/proc` inside the
  crannog:** a variable crannog adds itself, inherited by everything the
  session starts, and `list` would need a `container exec` per crannog.
- **Configurable proxy resources:** no need so far.

## Consequences

- `crannog list` counts sessions on the host, without entering the VM.
- Processes started in a crannog outside `crannog checkout` (e.g. a manual
  `container exec`) are not sessions and do not keep the crannog running.
- The proxy uses about a quarter of its memory limit under load.
