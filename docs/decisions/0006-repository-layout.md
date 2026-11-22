# 0006. Repository layout

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog ships two binaries: `crannog` and the git remote helper
`git-remote-crannog` ([ADR 0015](0015-git-causeway.md)), which git finds by
its name in `PATH`. More products, e.g. a macOS menu bar app, may follow.
The number of Cobra commands grows, and the end-to-end test needs Apple
Containers, which unit tests do not.

## Decision

- **One directory per binary** at the top level of one Go module; the
  module's root holds no Go package:

  ```
  go.mod                # module github.com/hettiger/crannog
  crannog/main.go       # package main: calls cmd.Execute()
  crannog/cmd/          # package cmd: Cobra commands, one file per command
  git-remote-crannog/   # package main: the git remote helper
  internal/             # packages shared by the binaries
  e2e/                  # end-to-end smoke test
  ```

- **Commands** are registered in `init()` of their file in `crannog/cmd/`.
- **Version:** injected into `crannog/cmd` at build time with `-ldflags`,
  defaulting to `dev`. The helper has no version.
- **Installing:** `go install github.com/hettiger/crannog/...@latest`
  installs both binaries. `make build` builds both into `dist/` for macOS
  on Apple Silicon ([ADR 0008](0008-apple-containers-runtime.md)).
- **End-to-end test:** `e2e/smoke.sh` runs on a Mac with Apple Containers
  against the real `~/.crannog` and removes the state of its temporary
  repository afterwards.

## Alternatives

- **`main.go` and `cmd/` in the root, the helper in its own directory:**
  only `crannog` is in the root, the helper is the exception.
- **Go's common layout `cmd/<binary>/` with Cobra in `internal/cli/`:**
  `cmd/` would no longer mean Cobra commands, and the commands move away
  from their binary.
- **Commands in `package main`:** `crannog/` grows with every command.
- **One binary that acts as the helper when called as
  `git-remote-crannog`:** `go install` cannot create the second name.
- **One module per binary and one for `internal/`:** the `...` pattern does
  not match packages in nested modules, so each binary needs its own
  `go install`, and every change to `internal/` needs its own tag and a
  version bump in both binaries' modules. It brings no benefit: the linker
  only includes the packages a binary imports.
- **A `CRANNOG_HOME` override or a temporary `HOME` for the smoke test:** a
  new feature, or untested with the `container` CLI.

## Consequences

- `go install github.com/hettiger/crannog@latest` installs nothing; a single
  binary is installed with its directory, e.g.
  `github.com/hettiger/crannog/crannog@latest`.
- Products in other languages get their own top-level directory.
- One `go.mod` lists the dependencies of both binaries.
- The smoke test leaves the user's agent homes and approved setups as any
  crannog would.
