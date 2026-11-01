# 0028. Cobra commands in a subpackage

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0027](0027-repository-layout.md) puts the entrypoint and the Cobra
commands of `crannog` together in `package main`. Separating the commands from
the entrypoint keeps `crannog/` small as the number of commands grows. Splitting
the repository into one Go module per binary was also considered, so that
`git-remote-crannog` does not carry the dependencies of `crannog`.

## Decision

- **Commands in `crannog/cmd/`:** `package cmd`, one file per command,
  registered in `init()`. `crannog/main.go` only calls `cmd.Execute()`.

  ```
  go.mod                # module github.com/hettiger/crannog
  crannog/main.go       # package main: calls cmd.Execute()
  crannog/cmd/          # package cmd: Cobra commands
  git-remote-crannog/   # package main: the git remote helper
  internal/             # packages shared by the binaries
  ```

- **Version:** injected into `crannog` with
  `-ldflags "-X github.com/hettiger/crannog/crannog/cmd.version=..."`,
  defaulting to `dev`. The helper has no version for now.
- **One module** for the whole repository stays. Installing is unchanged:
  `go install github.com/hettiger/crannog/...@latest`.

## Alternatives

- **One module per binary and one for `internal/`:** the `...` pattern does
  not match packages in nested modules, so each binary needs its own
  `go install` command. `go install pkg@version` rejects `replace`
  directives, so every change to `internal/` needs its own tag and a version
  bump in both binaries' modules, with prefixed tags such as
  `crannog/v0.1.0`. It brings no benefit: the linker only includes the
  packages a binary imports, so the helper contains no Cobra either way.

## Consequences

- This supersedes the `package main` commands and the version flag of
  ADR 0027. The directory per binary and the installation stay.
- One `go.mod` lists the dependencies of both binaries.
