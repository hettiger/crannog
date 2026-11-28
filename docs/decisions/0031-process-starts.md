# 0031. Process starts

- **Status:** Accepted
- **Date:** 2026-11-23

## Context

Seatbelt confines where crannog and the programs it runs can write files
([ADR 0030](0030-seatbelt.md)). It does not confine what crannog asks them
to do:

- The daemon of Apple Containers is not confined. `container` can delete
  any container, image or volume of the user and mount any host directory.
- git may write in the common git directory and to the network. Seatbelt
  cannot tell crannog's remote from the user's branches and remotes.

With commands built from free arguments all over the code, knowing what
crannog makes these programs do means reviewing every call. The reviewer
should be able to read one file per program instead, as with package
`hostfs` for file access ([ADR 0013](0013-host-file-access.md)).

## Decision

- Every program crannog runs on the host is started in one file: git in
  `internal/git/git.go`, `container` in `internal/runtime/runtime.go`,
  `ssh` in `internal/openssh/openssh.go`, `sandbox-exec` in
  `internal/seatbelt/seatbelt_darwin.go`.
- These packages offer one function per operation. None takes free
  arguments for the program on the host, and none returns a command for
  the caller to run.
- The git functions that write refuse what is not crannog's: they accept
  only remotes named `crannog-<slug>` with the URL `crannog::<slug>`. A
  value that becomes an argument of its own is refused if it starts with
  `-`, for git as for ssh's host name.
- A unit test fails if a file starts a process or opens a network
  connection and is not on a list of allowed files, each with a reason:
  it checks the imports of `os/exec`, `net`, `net/http` and
  `golang.org/x/sys/unix`, and the uses of `os.StartProcess`,
  `syscall.Exec`, `syscall.ForkExec` and `syscall.StartProcess`. Tests
  follow the same rule.
- Tests run git through package `testgit`. It passes any arguments, but
  runs git only in a directory below the temporary directory and refuses
  the options that make git work elsewhere (`-C`, `--git-dir`,
  `--work-tree`). Other programs tests run (`sh`, `cat`) are on the list
  file by file.
- In tests, package `runtime` refuses a `container` that is not below the
  temporary directory, so tests run a fake and never reach the user's
  containers, images and volumes. Package `openssh` does the same for
  `ssh`, which would connect to the user's hosts.

## Alternatives

- **A rule on the calls instead of the imports:** a package could still
  hand out a command, and the place that runs it would decide its
  arguments, files and environment. Without the import no file can do
  that.
- **A typed git API for the tests too:** the tests prepare repositories
  in many ways, with about a hundred calls. Where they can write is what
  matters, and that is bound to the temporary directory.
- **Checking the dependencies as well:** the test reads only crannog's own
  code. A fixed list of dependencies would make every update harder and
  still not say what a dependency does.
- **Leaving it to review:** a new `exec.Command` in an unrelated change is
  easy to miss.

## Consequences

- `git.go` and `runtime.go` show everything crannog makes git and
  `container` do on the host. A new operation is a new function there.
- `container exec` runs any command, but in a container: the commands are
  given by the callers, the options on the host by `runtime.go`.
- A new program on the host needs a place of its own and an entry in the
  list.
- Dependencies are not covered. For example, the text fields of the
  library crannog asks questions with read the clipboard with `pbpaste`
  when the user presses Ctrl+V in the filter of a selection. A new
  dependency is visible in `go.mod`
  ([ADR 0007](0007-go-dependencies.md)).
- The programs themselves run others: git runs the remote helper, a
  pager and the hooks of the user's repository. They inherit Seatbelt.
