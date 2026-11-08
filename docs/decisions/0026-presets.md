# 0026. Presets

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

A repository should work with crannog right away, without committing a
`.crannog/` first, and an agent inside a crannog is well placed to build a
setup that fits the project. Users also want their own starting points for
kinds of projects they use often.

## Decision

- **Presets:** a [preset](../glossary.md#preset) is a named template for
  `.crannog/` (`config.yml`, `iron-proxy.yaml`, `container/`) and nothing
  else. crannog ships `default`. Users can add presets in
  `~/.crannog/presets/<name>/`; a user preset replaces a bundled preset of
  the same name completely.
- **Fallback:** if the source of `.crannog/` has none
  ([ADR 0023](0023-repository-configuration.md)), crannog uses the preset
  `default`. Writing a preset into the repository is an explicit command.
- **No approval for presets:** presets live only on the host and no agent can
  reach them. crannog stores the preset it uses as an approved setup like any
  other, so later changes to `.crannog/` are compared with it.
- **Agents build setups:** the managed instructions block
  ([ADR 0029](0029-managed-instructions-block.md)) explains `.crannog/`, so
  the user can ask an agent to build a setup for the repository. It reaches
  the user like any other work: fetch, merge, approval diff on the next
  session or apply.

## Alternatives

- **Writing a default `.crannog/` when setting up a repository:** every
  repository needs a step before the first crannog, and the files are
  committed before anyone knows what the project needs.
- **Presets that also change other paths (e.g. `.claude/`):** instructions
  for agents come from the managed instructions block; merging other paths
  adds failure modes.

## Consequences

- `crannog checkout` works in any repository without preparation.
- Changing a user preset changes the crannogs that use it on their next
  session or apply, without an approval diff.
