# 0001. Go and Cobra for the CLI

- **Status:** Accepted
- **Date:** 2026-10-30

## Context

crannog is a command line application and needs a language and a CLI framework.

## Decision

crannog is written in Go (module `github.com/hettiger/crannog`) and uses
`github.com/spf13/cobra` for commands, flags, help, and shell completion.
Commands live in `cmd/`, one file per command. The version is injected at
build time via
`-ldflags "-X github.com/hettiger/crannog/cmd.version=..."` and defaults to `dev`.

## Alternatives

None evaluated.

## Consequences

- Single static binary, easy distribution.
- Cobra provides `help` and `completion` commands out of the box.
