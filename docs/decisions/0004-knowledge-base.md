# 0004. Knowledge base in `docs/`

- **Status:** Accepted
- **Date:** 2026-10-30

## Context

Context about terms, decisions, features, and use cases gets lost between
work sessions, with humans and coding agents alike, and has to be repeated.

## Decision

`docs/` is the project knowledge base. It contains a glossary, ADRs, feature
docs, and use case docs. `AGENTS.md` at the repository root tells coding agents
to read the knowledge base and to maintain it as part of every change.
Glossary terms are agreed on with the maintainer before they are recorded.

## Alternatives

- Wiki outside the repository: drifts from the code, not versioned with it.
- Agent-private memory: not visible to other contributors.

## Consequences

- Docs are versioned and reviewed together with the code.
- Every change may require a docs update.
