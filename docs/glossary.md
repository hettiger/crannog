# Glossary

Agreed terms used throughout crannog: in code, docs, and discussions.
Entries are sorted alphabetically.

<!--
Entry format:

## Term

Definition in one or two sentences.

- **Not to be confused with:** related term (optional)
- **In code:** identifier(s) used for this concept (optional)
-->

## ADR

Architecture Decision Record. A short document in `docs/decisions/` that
records one significant decision, its context, and its consequences.

## Agent

The coding agent that works autonomously inside a crannog.

## Agent home

The directory `~/.crannog/agents/<agent>` on the host, shared by all crannogs.
It holds what the agent otherwise keeps in the user's home: login, settings,
history.

- **Not to be confused with:** the user's own agent configuration (e.g.
  `~/.claude` on the host), which crannog does not touch

## Causeway

A connection between the host and a crannog, configured and controlled by the
user. Causeways are the only way into and out of a crannog. Kinds: git
causeway, proxy causeway, mount causeway.

- **Not to be confused with:** bridge (not used as a term)

## Crannog

An isolated sandbox in which an agent works. Reachable only via causeways.

- **Not to be confused with:** crannog, the CLI (written in lowercase)

## Git causeway

The causeway through which code enters a crannog and the agent's results
(commits, branches) leave it. Every crannog has one.

## Mount causeway

An optional causeway that mounts a volume from the host into a crannog.

## Proxy causeway

An optional causeway that gives a crannog controlled network access.

## User

The human who sets up crannogs and controls their causeways.
