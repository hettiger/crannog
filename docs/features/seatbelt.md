# Seatbelt

- **Status:** Implemented
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0013](../decisions/0013-host-file-access.md), [0030](../decisions/0030-seatbelt.md)

## Summary

`crannog` and `git-remote-crannog` run in the macOS sandbox and can write
files only where crannog has to. On macOS the tests run in it as well.

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

## Tests

- On macOS, `make test` (and so `make check`) first runs
  `go mod download` and then `go test ./...` through `sandbox-exec` with
  `internal/seatbelt/test.sb`. Writing files is allowed only in the
  user's temporary directory (`TEMP_DIR`), the Go build cache
  (`GO_CACHE`) and the devices above.
- On macOS, every test binary runs the self-test once when it starts,
  in the user's real home directory. Outside of Seatbelt it fails before
  any test runs, with `on macOS, tests must run in Seatbelt (make test)`.
- Every package with tests has a `selftest_test.go` that imports package
  `selftest` blank; its `init` runs the self-test.
  `TestEveryPackageWithTestsImportsTheSelfTest` in `internal/rules` fails
  for a package with tests that does not. It reads the packages itself, not through
  `go list`, so the test cache notices a `selftest_test.go` that was
  added or deleted.
- On other systems the tests run without Seatbelt.
