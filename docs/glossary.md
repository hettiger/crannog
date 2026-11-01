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

A coding agent (e.g. Claude Code, Codex, OpenCode, Pi) that works
autonomously inside a crannog. Several agents can be installed in one
crannog and call each other.

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

## Confirmation

A yes/no question crannog asks before an action whose consequences the user
might not expect, e.g. destroying a crannog, replacing `.crannog/` or ending
running sessions. It never decides which setup a crannog uses. `--yes`
answers it with yes; without a terminal and without `--yes` it is an error.

- **Not to be confused with:** setup approval
- **In code:** `Confirmation`

## Crannog

An isolated sandbox in which an agent works. Reachable only via causeways.

- **Not to be confused with:** crannog, the CLI (written in lowercase)

## Git causeway

The causeway through which code enters a crannog and the agent's results
(commits, branches) leave it. Every crannog has one.

## Mount causeway

An optional causeway that mounts a volume from the host into a crannog.

## Preset

A named template for a repository's `.crannog/` directory. crannog ships a
`default` preset and uses it for repositories without `.crannog/`; users can
add their own in `~/.crannog/presets/<name>/`.

- **Not to be confused with:** a repository's own `.crannog/`, which may
  start from a preset but is then versioned with the repository
- **In code:** `Preset`

## Proxy causeway

The causeway through which a crannog reaches the network: a proxy that
allows only listed domains and injects credentials. Every crannog has one.

## Repository

The user's git repository on the host from which crannogs are built. All of
its git worktrees count as one repository. Its configuration lives in
`.crannog/`; crannog keeps its state about it in
`~/.crannog/repositories/<id>/`.

- **Not to be confused with:** the copy of the repository inside a crannog
  (on the crannog's volume)
- **In code:** `Repository`

## Session

An interactive process the user starts in a crannog, e.g. an agent or a
shell. What it runs is configured in `.crannog/config.yml`.

- **Not to be confused with:** an agent's own sessions (conversations it
  stores, e.g. for resuming)

## Setup approval

The user's agreement to use a setup, i.e. a content of `.crannog/`, in
crannogs, given after seeing a diff. Each content is approved once; crannog
keeps approved copies in `approved/<sha256>/`. `--yes` never skips a setup
approval. Short: approval.

- **Not to be confused with:** confirmation; an agent's own approvals or
  permission prompts
- **In code:** `SetupApproval`

## Start branch

The branch of the user's repository a crannog is built from and identified
by. A repository has at most one crannog per start branch. It is checked out
in the crannog; the agent may create further branches there.

- **In code:** `StartBranch`

## User

The human who sets up crannogs and controls their causeways.
