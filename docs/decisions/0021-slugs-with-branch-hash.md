# 0021. Slugs with a branch hash

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog derives a slug from the start branch and uses it in the runtime
names, the remote and the host directory
([ADR 0014](0014-one-crannog-per-start-branch.md)). The slug rule of
[ADR 0012](0012-crannog-names-volumes-paths.md) maps `feature/x` and
`feature-x` to the same slug, so ADR 0014 refuses the second one with an
error the user has to understand.

Apple Containers 1.6.1 limits container and network names to 63
characters. The longest runtime name, `crannog-<id8>-<slug>-proxy`, leaves
40 characters for the slug; long branch names exceed that.

## Decision

- **Slug:** the start branch cleaned with the rule of ADR 0012, cut to its
  first 31 characters without a trailing `-`, followed by `-` and the first
  eight hex characters of the sha256 of the branch name. A slug has at most
  40 characters. Examples:

  ```
  feature/x                                            feature-x-217d2bf5
  feature-x                                            feature-x-c791eb83
  feature/JIRA-1234-implement-the-new-billing-export   feature-jira-1234-implement-the-41ed7bc7
  ```

- Every slug contains the hash, also short ones.
- **Safety net:** if `crannogs/<slug>/branch` names another branch, crannog
  refuses with an error. This replaces ADR 0014's error for branches with the
  same slug as the expected case.

A [spike](0022-default-image-packages-and-state/README.md) measured the limits on `container` 1.6.1: 63 characters for
containers and networks, 255 for volumes.

## Alternatives

- **A hash only for long slugs:** keeps short names readable, but
  `feature/x` and `feature-x` still collide.
- **Truncating without a hash:** long branches with the same beginning
  collide.
- **The hash alone:** names no longer show the branch.

## Consequences

- Branches never collide in practice; the user needs no rule about it.
- Remote names are longer, e.g. `crannog-feature-x-217d2bf5/feature/x`, but
  exist only while the crannog runs.
- Runtime names stay within the limits for any branch name.
