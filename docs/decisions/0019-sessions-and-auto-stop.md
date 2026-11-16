# 0019. Sessions and auto-stop

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

Every crannog is a VM that uses memory while it runs. A crannog left running
by mistake is easy to forget. To stop a crannog when it is no longer used,
crannog must know how many [sessions](../glossary.md#session) run in it,
also when crannog itself is killed. Counting inside the crannog would need
an environment variable crannog adds to each session, which
[ADR 0025](0025-configuration-schema.md) rules out, and processes a session
starts inherit it.

## Decision

- **Sessions:** several sessions can run in one crannog at the same time.
  They share the working tree and everything running in the crannog. Before
  starting a session while another one runs, crannog asks for confirmation.
- **Auto-stop:** when the last session ends, however it ends, crannog stops
  the crannog and its proxy and removes its remote. Background processes end
  with it. The next session starts the crannog again. There is no stop
  command.
- **Sessions are counted on the host:** the crannog's host directory
  ([ADR 0021](0021-crannog-state-on-the-host.md)) has a `lock` file and a
  `sessions/` directory.
  - Starting: crannog takes an exclusive `flock` on `lock`, starts the
    crannog if needed, creates `sessions/<pid>` with its own process id and
    takes an exclusive `flock` on it. Then it releases `lock` and runs the
    session with `container exec`, which inherits the file descriptor. If
    crannog is killed, the lock lasts as long as the session runs.
  - Counting: crannog tries to lock each file in `sessions/` without
    waiting. A file it cannot lock is a running session. A file it can lock
    belongs to an ended session and is deleted.
  - Ending: crannog deletes its file, takes `lock` and counts the sessions.
    If none is left, it stops the crannog. Start and stop both hold `lock`,
    so they cannot race.
- **Self-repair:** `checkout`, `apply`, `destroy` and `list` first stop
  running crannogs of the repository with no session whose `lock` nobody
  holds, and remove `crannog-*` remotes with a `crannog::` URL whose crannog
  is not running. The preset commands, which work without the runtime, do
  not.

## Alternatives

- **Keeping crannogs running after the last session:** forgotten crannogs use
  memory until someone stops them.
- **Lock files without `flock`:** stay behind when crannog is killed; the
  kernel releases a `flock` when the process ends.
- **One shared lock for all sessions:** shows whether sessions run, not how
  many.
- **An environment variable per session, counted in `/proc` inside the
  crannog:** a variable crannog adds itself, inherited by everything the
  session starts, and `list` would need a `container exec` per crannog.

## Consequences

- `crannog list` counts sessions on the host, without entering the VM.
- Background services in a crannog only run while a session does.
- Loops that start a session per iteration start and stop the VM each time.
- Processes started in a crannog outside `crannog checkout` (e.g. a manual
  `container exec`) are not sessions and do not keep the crannog running.
- If crannog is killed after its session ended, the crannog keeps running
  until the next crannog call in the repository.
