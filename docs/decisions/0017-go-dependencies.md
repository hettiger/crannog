# 0017. Go dependencies

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

Implementing [ADR 0016](0016-cli-commands.md) needs atomic file writes, YAML,
terminal detection, prompts, tables and tests. crannog is MIT licensed and
should depend on few, well maintained libraries.

## Decision

Direct dependencies besides Cobra ([ADR 0001](0001-go-and-cobra.md)):

| Module | Used for | License |
| ------ | -------- | ------- |
| `github.com/google/renameio/v2` | Atomic file writes | Apache-2.0 |
| `go.yaml.in/yaml/v3` | `config.yml` | MIT and Apache-2.0 |
| `golang.org/x/term` | Detecting terminals | BSD-3-Clause |
| `charm.land/huh/v2` | Confirmations, approvals, pickers | MIT |
| `charm.land/lipgloss/v2` | Tables and styling | MIT |
| `github.com/stretchr/testify` | Tests (`require`, `assert`) | MIT |

Everything else uses the standard library or the tools crannog already
requires:

- git and `container` are called with `os/exec`, not through libraries.
- The approval diff is `git diff --no-index` on the two directories.
- Locks use `syscall.Flock`; the bundled presets use `embed`.
- Fakes for tests are plain interfaces, not generated mocks.

## Alternatives

- **`gopkg.in/yaml.v3`:** archived since April 2025; `go.yaml.in/yaml/v3` is
  its maintained successor with the same API.
- **`text/tabwriter`:** tables look worse than with Lip Gloss.
- **A Go diff library or go-git:** would behave differently from the git the
  user knows.

## Consequences

- All dependencies are permissively licensed and compatible with the MIT
  license. Release binaries must ship their license texts and NOTICE files;
  `make THIRD_PARTY_NOTICES` collects them with `go-licenses`.
- Bubble Tea and other Charm modules come in as indirect dependencies.
