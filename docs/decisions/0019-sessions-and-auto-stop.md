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
- **Ending sessions:** before `destroy` or a rebuild
  ([ADR 0022](0022-rebuilds.md)) deletes the containers, crannog sends
  SIGTERM to the processes of the agent user in the crannog and waits until
  no session runs, without a timeout. The programs restore their terminals
  themselves. Processes that ignore SIGTERM, as interactive shells do, get
  SIGHUP once they have no child process left, so a program in a shell's
  foreground ends first. Nothing is deleted before, so Ctrl-C leaves the
  crannog as it was.
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

- **Deleting the containers under running sessions:** the programs are
  killed and leave mouse reporting and keyboard modes on in the terminal.
  Resetting the terminal from the host means guessing what they turned on.
- **Killing sessions that do not end after a timeout:** the terminal is left
  broken after all; waiting shows what crannog waits for, and the user ends
  the session or aborts.
- **SIGHUP for every process:** programs that clean up on SIGTERM but not on
  SIGHUP are killed before they restored the terminal.

## Consequences

- A session whose program ends on neither SIGTERM nor SIGHUP blocks
  `destroy` and rebuilds until the user ends it.
- `crannog list` counts sessions on the host, without entering the VM.
- Background services in a crannog only run while a session does.
- Loops that start a session per iteration start and stop the VM each time.
- Processes started in a crannog outside `crannog checkout` (e.g. a manual
  `container exec`) are not sessions and do not keep the crannog running.
- If crannog is killed after its session ended, the crannog keeps running
  until the next crannog call in the repository.
