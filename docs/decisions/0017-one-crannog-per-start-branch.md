# 0017. One crannog per start branch

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

The user works in the context of a repository and a branch and needs to
address crannogs without learning their names. Crannog names in the CLI
would need rules for explicit names, derived names, conflicts and
selection, which makes the CLI hard to explain. A second crannog from the
same branch is rarely needed and is one `git branch` away.

Names of containers, volumes and networks are global in Apple Containers.
Container and network names are limited to 63 characters.

## Decision

- **Identity:** a repository has at most one crannog per
  [start branch](../glossary.md#start-branch). Commands identify crannogs by
  their start branch (`[branch]`, defaulting to the checked-out branch).
  There are no crannog names in the CLI.
- **Branch slug:** crannog derives a slug from the start branch and uses it
  only internally: in the remote name, the runtime names and the host
  directory. The branch is lowercased, every character outside `a-z0-9`
  becomes `-`, repeated dashes are collapsed and leading or trailing ones
  removed; the result is cut to 31 characters without a trailing `-` and
  followed by `-` and the first eight hex characters of the sha256 of the
  branch name. If nothing is left of the branch, the slug is the hash
  alone. A slug has at most 40 characters:

  ```
  feature/x                                            feature-x-217d2bf5
  feature-x                                            feature-x-c791eb83
  feature/JIRA-1234-implement-the-new-billing-export   feature-jira-1234-implement-the-41ed7bc7
  ü                                                    607474ca
  ```

- **Safety net:** if the host directory of a slug names another branch,
  crannog refuses with an error.
- **Runtime names** start with the first eight characters of the
  repository id ([ADR 0021](0021-crannog-state-on-the-host.md)):

  ```
  crannog-<id8>-<slug>          # crannog
  crannog-<id8>-<slug>-proxy    # proxy
  crannog-<id8>-<slug>-net      # internal network
  crannog-<id8>-<slug>-work     # volume at /work
  crannog-<id8>-<slug>-state    # volume at /state
  ```

A [spike](../spikes/default-image-and-network/README.md) measured the name
limits on `container` 1.6.1: 63 characters for containers and networks, 255
for volumes.

## Alternatives

- **Crannog names and several crannogs per branch:** most of the CLI's rules
  would exist only for this case.
- **Appending a number on name conflicts:** creates crannogs the user does
  not know about.
- **A slug without hash, refusing branches with the same slug:**
  `feature/x` and `feature-x` collide, and the user has to understand why.
- **A hash only for long slugs:** short names collide as above.
- **Truncating without a hash:** long branches with the same beginning
  collide.
- **The hash alone:** names no longer show the branch.
- **A fixed prefix such as `branch-` for branches without ASCII letters or
  digits:** suggests a branch of that name. Refusing them is no option:
  they are valid git branches.

## Consequences

- Users address crannogs like branches and never pick a name.
- A second crannog for the same branch needs a new branch.
- Crannogs of different repositories never collide, and runtime names stay
  within the limits for any branch name.
- Remote names are long, e.g. `crannog-feature-x-217d2bf5/feature/x`, but
  exist only while the crannog runs.
