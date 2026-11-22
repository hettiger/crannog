# 0024. Setup approval

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

The repository also lives inside the crannog
([ADR 0015](0015-git-causeway.md)). An agent can change and commit its
`.crannog/`, and the user may merge that change without noticing. A new
mount such as the user's home would break the crannog's isolation.

## Decision

- **Approval before use:** crannog only uses
  [approved setups](../glossary.md#approved-setup). If a setup differs from
  every approved one, crannog shows a diff of all files
  (`git diff --no-index`) and asks before it builds or applies it. A new
  crannog is compared with the most recent approval, an existing one with
  its own setup; the very first approval shows everything. The approval
  also lists the variables the setup reads from the host environment
  ([ADR 0025](0025-configuration-schema.md)).
- **Approved once, approved always:** each setup, identified by its
  [setup hash](../glossary.md#setup-hash), is approved once, whichever
  branch it comes from.
- **Approved setups are stored** in `approved/<sha256>/`
  ([ADR 0021](0021-crannog-state-on-the-host.md)); crannog builds from these
  copies, never from the source.
- **Presets** need no approval ([ADR 0026](0026-presets.md)); the setup
  crannog takes from one is stored as an approved setup like any other.
- **Never skipped:** `--yes` does not answer a setup approval. Without a
  terminal, a needed approval is an error. Errors in `config.yml` and in
  the host environment are reported before any approval.

## Alternatives

- **Approval by hash only:** a hash shows that something changed, not what.
- **One approval per repository path:** does not work with branches that have
  different setups.
- **Approving every difference to the crannog's setup:** asks again for
  content the user has already seen.
- **An extra confirmation when uncommitted changes are not used:** the
  approval already asks whenever something changes.

## Consequences

- Changes to `.crannog/`, including the agent's, never take effect without the
  user seeing a diff.
- Merging an approved setup between branches needs no second approval.
- Scripts cannot use a new setup; the user approves it once in a terminal.
