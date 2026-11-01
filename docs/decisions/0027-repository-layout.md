# 0027. Repository layout with one directory per binary

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0001](0001-go-and-cobra.md) puts the Cobra commands in the package
`cmd/` and `main.go` in the module's root. crannog also needs a second
binary, the git remote helper `git-remote-crannog`
([ADR 0010](0010-git-causeway.md)), which git finds by its name in `PATH`.
More products, e.g. a macOS menu bar app, may follow.

## Decision

- **One directory per binary** at the top level of the module:

  ```
  crannog/              # package main: entrypoint and Cobra commands
  git-remote-crannog/   # package main: the git remote helper
  internal/             # packages shared by the binaries
  ```

  The module's root holds no Go package. In `crannog/`, Cobra commands keep
  one file per command.
- **Version:** injected into both binaries with
  `-ldflags "-X main.version=..."`, defaulting to `dev`. This replaces the
  package path of ADR 0001.
- **Installing:** `go install github.com/hettiger/crannog/...@latest`
  installs both binaries. `make build` builds both into `dist/` for macOS on
  Apple Silicon ([ADR 0006](0006-apple-containers-runtime.md)).

## Alternatives

- **`main.go` and `cmd/` in the root, the helper in its own directory:**
  only `crannog` is in the root, the helper is the exception.
- **Go's common layout `cmd/<binary>/` with Cobra in `internal/cli/`:**
  `cmd/` would no longer mean Cobra commands, and the commands move away
  from their binary.
- **One binary that acts as the helper when called as
  `git-remote-crannog`:** `go install` cannot create the second name.

## Consequences

- `go install github.com/hettiger/crannog@latest` installs nothing; a single
  binary is installed with its directory, e.g.
  `github.com/hettiger/crannog/crannog@latest`.
- Products in other languages get their own top-level directory.
