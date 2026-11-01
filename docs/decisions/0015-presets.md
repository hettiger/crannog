# 0015. Presets

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0011](0011-repository-configuration.md) has crannog write a default
`.crannog/` when it sets up a repository. A repository should work with
crannog right away, and an agent inside a crannog is well placed to build a
setup that fits the project. Users also want their own starting points for
kinds of projects they use often.

ADR 0011 reads `.crannog/` from the working tree if the start branch is
checked out. With several git worktrees, that working tree may not be the one
the user is looking at, and uncommitted changes in the current worktree may
not be the ones that are used.

## Decision

- **Presets:** a [preset](../glossary.md#preset) is a named template for
  `.crannog/` (`config.yml`, `iron-proxy.yaml`, `container/`) and nothing
  else. crannog ships `default`. Users can add presets in
  `~/.crannog/presets/<name>/`; a user preset replaces a bundled preset of
  the same name completely.
- **Fallback:** if the source of `.crannog/` has none, crannog uses the preset
  `default`. This replaces writing a default when setting up a repository
  (ADR 0011). Writing a preset into the repository is an explicit command.
- **No approval for presets:** presets live only on the host and no agent can
  reach them. crannog stores the preset it uses as an approved copy like any
  other, so later changes to `.crannog/` are compared with it.
- **Source of `.crannog/`:** crannog reads the working tree only if the start
  branch is checked out in the current worktree. Otherwise it reads the start
  branch's latest commit. This narrows the rule of ADR 0011. crannog always
  prints the source (e.g. `from feature/x (commit a1b2c3d)`, `from working
  tree (uncommitted changes)`, `from preset default`), also above an approval
  diff. If the current worktree has uncommitted changes to `.crannog/` that
  are not used, crannog says so.
- **Agents build setups:** the managed instruction block
  ([ADR 0013](0013-agents-default-image.md)) explains `.crannog/`, so the
  user can ask an agent to build a setup for the repository. It reaches the
  user like any other work: fetch, merge, approval diff on the next session or
  apply.

## Alternatives

- **Writing a default on setup:** every repository needs a step before the
  first crannog, and the files are committed before anyone knows what the
  project needs.
- **Presets that also change other paths (e.g. `.claude/`):** instructions
  for agents come from the managed block; merging other paths adds failure
  modes.
- **Reading the working tree of any worktree:** uses uncommitted changes the
  user does not see from where they are.
- **An extra confirmation when uncommitted changes are not used:** the
  approval already asks whenever something changes.

## Consequences

- `crannog checkout` works in any repository without preparation.
- Users who change `.crannog/` in one worktree and apply it to a branch
  checked out elsewhere must commit first.
- Changing a user preset changes the crannogs that use it on their next
  session or apply, without an approval diff.
