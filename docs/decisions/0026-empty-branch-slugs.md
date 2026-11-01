# 0026. Branch slugs of branches without ASCII letters or digits

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

A [branch slug](../glossary.md#branch-slug) is the cleaned start branch,
cut to 31 characters, followed by `-` and eight hex characters of the
branch's sha256 ([ADR 0021](0021-slugs-with-branch-hash.md)). Cleaning
replaces everything outside `a-z0-9` and removes leading and trailing dashes
([ADR 0012](0012-crannog-names-volumes-paths.md)). A branch without ASCII
letters or digits, e.g. `ü`, is cleaned to an empty string, which would give
the slug `-607474ca`.

## Decision

If the cleaned branch is empty, the branch slug is the hash alone, e.g.
`607474ca` for `ü`.

## Alternatives

- **A fixed prefix such as `branch-`:** suggests a branch of that name.
- **Refusing such branches:** they are valid git branches.

## Consequences

- Branch slugs never start with a dash; names built from them stay valid.
