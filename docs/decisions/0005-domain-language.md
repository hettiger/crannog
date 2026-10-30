# 0005. Domain language

- **Status:** Accepted
- **Date:** 2026-10-30

## Context

The name crannog suggests a metaphor: an isolated island dwelling, reachable
only via a causeway. Metaphors give a strong brand but can make a project harder
to understand if overused.

## Decision

Use the metaphor only where it names a real concept:

- **Crannog** for the sandbox an agent works in.
- **Causeway** for a user-controlled connection between host and crannog
  (git, proxy, mount causeway).

Everything else uses plain terms: **agent**, **user**. Creating and removing a
crannog is called **build** and **destroy** in prose. These are not command
names; command naming is decided separately.

See [glossary](../glossary.md) for definitions.

## Alternatives

- **Bridge** instead of causeway: immediately understandable, but weaker brand
  and historically inaccurate.
- **Keeper** instead of user: rejected, adds vocabulary without adding meaning.
- **Build / raze** as lifecycle terms: rejected.

## Consequences

Contributors only need to learn two project-specific terms.
