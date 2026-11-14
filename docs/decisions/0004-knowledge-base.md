# 0004. Knowledge base in `docs/`

- **Status:** Accepted
- **Date:** 2026-10-30

## Context

Context about terms, decisions, features, and use cases gets lost between
work sessions, with humans and coding agents alike, and has to be repeated.

## Decision

- `docs/` is the project knowledge base. It contains a glossary, ADRs,
  feature docs, use case docs and spikes. `AGENTS.md` at the repository
  root tells coding agents to read the knowledge base and to maintain it as
  part of every change. Glossary terms are agreed on with the maintainer
  before they are recorded.
- **ADRs record decisions**, one topic each: what was decided, why, and the
  alternatives worth remembering. Behavior and implementation details
  belong in feature docs or the code. The index in `docs/decisions/` groups
  ADRs by topic and says what each one decides.
- **Until the first release, ADRs are revised in place:** a changed
  decision is edited into the ADR it belongs to; ADRs are merged, split or
  renumbered when that keeps them atomic, and all references are updated.
  After the first release, an accepted ADR is no longer rewritten: a new
  ADR supersedes it.
- **Spikes** in `docs/spikes/` hold the scripts and results that confirmed
  assumptions of ADRs, so they can be rerun.

## Alternatives

- Wiki outside the repository: drifts from the code, not versioned with it.
- Agent-private memory: not visible to other contributors.
- Superseding ADRs from the start: before anything is released, chains of
  partially superseded ADRs make the current state hard to find.

## Consequences

- Docs are versioned and reviewed together with the code.
- Every change may require a docs update.
- Before the first release, an ADR's history is in git, not in the ADRs.
