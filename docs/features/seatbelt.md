# Seatbelt

- **Status:** In progress
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0013](../decisions/0013-host-file-access.md), [0030](../decisions/0030-seatbelt.md)

## Summary

`crannog` and `git-remote-crannog` run in the macOS sandbox and can write
files only where crannog has to.

## Usage

Nothing to do: both binaries confine themselves when they start. There is
no option to turn it off.

## Behavior

- On start, the binary replaces itself with
  `/usr/bin/sandbox-exec -p <profile> -D <parameter>... <binary> <arguments>`
  and sets `CRANNOG_SEATBELT=1` for it. The profile is
  `internal/seatbelt/profile.sb`.
- Reading files, running programs and the network are not restricted.
  Writing files is allowed only in:

  | Parameter      | Location                                                             |
  | -------------- | -------------------------------------------------------------------- |
  | `CRANNOG_HOME` | `~/.crannog`                                                         |
  | `TEMP_DIR`     | the user's temporary directory (`$TMPDIR`)                           |
  | `WORKTREE`     | in the current worktree: `.crannog` and `.crannog.tmp-*` only        |
  | `GIT_DIR`      | the common git directory of the current repository                   |
  | `BUILDER_DIR`  | `~/Library/Application Support/com.apple.container/builder`          |
  | –              | `/dev/null`, `/dev/tty`, `/dev/ttys*`, `/dev/dtracehelper`           |

- The locations are those of the working directory when the binary
  starts, with symlinks resolved. Outside of a repository there is no
  worktree and no git directory to write to. `~/.crannog` is created
  before entering if it is missing.
- With `CRANNOG_SEATBELT` set, the binary does not enter again and runs
  the self-test: it tries to create `~/.crannog-sandbox-check`.
  - "Operation not permitted": Seatbelt is active, the program continues.
  - The file was created: it is deleted and the program fails with
    `Seatbelt is not active`.
  - Any other error, such as a file of that name that exists: the program
    fails with `cannot check that Seatbelt is active`.
- `git-remote-crannog` run by `crannog` (through `git push`) inherits
  crannog's sandbox. Run by the user's git, it enters its own.
- Inside the sandbox of another program, entering fails with
  `sandbox-exec: sandbox_apply: Operation not permitted`.
- On other systems than macOS, both binaries fail.
