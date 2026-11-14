# 0030. Seatbelt

- **Status:** Accepted
- **Date:** 2026-11-14

## Context

crannog writes and deletes files on the host. [ADR 0013](0013-host-file-access.md)
confines that to anchors in code, with `os.Root` and a unit test. Relying
on it means reviewing every change to file access, and it does not cover
the programs crannog runs (`git`, `container`). The reviewer should be able
to rely on one small, fixed list of places crannog can write to.

## Decision

- `crannog` and `git-remote-crannog` confine themselves with Seatbelt, the
  macOS sandbox, before anything else runs: `main` replaces the process
  with `/usr/bin/sandbox-exec`, which runs the same binary with the same
  arguments. The kernel enforces the profile for the process and
  everything it runs, and it cannot be left.
- Both binaries share one profile and the code that enters it, package
  `seatbelt`. The profile allows everything except writing files. Writes
  are allowed only in:
  - the anchors: `~/.crannog`, the user's temporary directory, and
    `.crannog` and its temporary siblings in the current worktree
  - the common git directory of the current repository
  - the builder directory of Apple Containers,
    `~/Library/Application Support/com.apple.container/builder`
  - `/dev/null`, `/dev/dtracehelper` and the terminal devices
- The locations are fixed before entering and passed as parameters, with
  symlinks resolved, as Seatbelt compares resolved paths. Package `hostfs`
  resolves them.
- Every confined process runs a self-test first: creating
  `~/.crannog-sandbox-check` must fail with "operation not permitted". If
  the file can be created, it is deleted and the program fails. An
  environment variable marks a process started through `sandbox-exec`; it
  only saves entering again and is never trusted on its own.
- There is no way to turn Seatbelt off. On other systems than macOS the
  binaries fail.
- The end-to-end smoke test and the spikes are not confined.

## Alternatives

- **Allowing only `crannog-*` in the temporary directory:** `container
  build` writes an archive of the build context directly into the user's
  temporary directory and ignores `$TMPDIR`. A rule for the archive's name
  would depend on an internal of the runtime.
- **Denying reads too:** much more to allow and to maintain; the goal is
  that crannog cannot damage files.
- **A static rule for every directory named `.crannog`:** needs no
  locations before entering, but allows far more than the current
  worktree.
- **Running `git-remote-crannog` in a profile of its own:** two profiles
  to review. Seatbelt also refuses to enter a second, different profile,
  and `crannog` runs the helper through `git push`.
- **Trusting the environment variable alone:** anyone can set it. The
  self-test fails then.
- **A switch to turn Seatbelt off:** then nobody can rely on it.
- **Confining the smoke test and the spikes:** a script cannot be confined
  as a whole when it runs `crannog`, and what would remain is its own
  `rm -rf` of a directory it checks itself.

## Consequences

- A bug in crannog's file access, or in `git` and `container` as crannog
  runs them, cannot write outside of the list above.
- The runtime's daemon is not confined: what a guest can write to on the
  host follows from the mounts crannog passes, which stay a matter of
  review ([ADR 0014](0014-host-stays-out-of-guest-writable-directories.md)).
- The temporary directory is writable as a whole. That crannog only
  creates `crannog-*` directories in it is enforced by `hostfs`, not by
  Seatbelt.
- Seatbelt enters a second time only with the identical profile and
  parameters. `git-remote-crannog` therefore does not enter again when
  `crannog` runs it, and crannog cannot run inside the sandbox of another
  program: `sandbox-exec` fails with `sandbox_apply: Operation not
  permitted`.
- The self-test shows that the home directory is not writable, not that
  crannog's profile is the active one: with the variable set inside
  another program's sandbox, that sandbox's rules apply.
- Entering replaces the process, so unit tests cover the profile, the
  arguments and the self-test, but not Seatbelt itself. That is checked by
  the [spike](../spikes/seatbelt/README.md) and the smoke test.
- `sandbox-exec` is deprecated by Apple but still what other tools use. An
  update of macOS or of Apple Containers can make crannog fail until the
  profile is adjusted; rerun the spike then.
