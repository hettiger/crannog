# 0023. Git author from the host

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

The agent returns its results as commits through the git causeway
([ADR 0010](0010-git-causeway.md)). git refuses to commit without
`user.name` and `user.email`. The default image
([ADR 0013](0013-agents-default-image.md)) sets neither, and the user may use
a different author per repository on the host, e.g. through `includeIf`.

## Decision

- **Before every session**, together with the `/etc/hosts` entry
  ([ADR 0020](0020-proxy-address.md)), crannog reads `git config user.name`
  and `git config user.email` in the user's repository on the host, from the
  current directory, so `includeIf` and the repository's own config apply.
- crannog sets both values in the local config of the repository in the
  crannog (`/work/<host path>/.git/config`), as the agent user. Values are
  passed as arguments, never through a shell.
- A value that is not set on the host is removed in the crannog as well, and
  crannog prints a warning. The crannog always mirrors the host.

## Alternatives

- **Copying the author once, on the initial transfer:** a change on the host
  needs a new crannog.
- **A git config file per repository on the host, mounted into crannogs:**
  needs another mount and variable in `config.yml` and a file next to the
  user's own git config.
- **A fixed agent identity:** commits would need rewriting before the user
  merges them as their own.
- **`GIT_AUTHOR_*` and `GIT_COMMITTER_*` via `env` in `config.yml`:** these
  variables are usually not set on the host, and an unset variable is an
  error ([ADR 0019](0019-configuration-schema.md)).

## Consequences

- Commits in a crannog carry the author the user has on the host for this
  repository. Setting another author for the repository on the host takes
  effect with the next session.
- All worktrees in the crannog share the author.
- An author the agent sets in the repository's config lasts until the next
  session.
