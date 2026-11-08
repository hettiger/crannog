# Glossary

Agreed terms used throughout crannog: in code, docs, and discussions.
Entries are sorted alphabetically.

<!--
Entry format:

## Term

Definition in one or two sentences.

- **Not to be confused with:** related term (optional)
- **In code:** identifier(s) used for this concept (optional)
- **In earlier ADRs:** names used before the term was agreed (optional)
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

## Anchor

A directory crannog opens as the root of all file access below it:
`~/.crannog`, a worktree's `.crannog/`, or a temporary directory crannog
created ([ADR 0031](decisions/0031-host-file-access.md)).

- **In code:** package `hostfs`

## Approved setup

A setup the user approved, or a preset crannog used, stored by its setup
hash in `~/.crannog/repositories/<id>/approved/<sha256>/`. Crannogs are built
only from approved setups.

- **Not to be confused with:** setup approval, the user's agreement that
  makes a setup an approved setup
- **In code:** `setup.Store`
- **In earlier ADRs:** approved copy (ADR 0011, 0014, 0015, 0019)

## Branch slug

The identifier crannog derives from a start branch: the cleaned branch name
and a hash of it. Used internally for runtime names, the remote and the host
directory; users never type it.

- **In code:** `BranchSlug`

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
- **In code:** `ui.UI.AskConfirmation`

## Crannog

An isolated sandbox in which an agent works. Reachable only via causeways.

- **Not to be confused with:** crannog, the CLI (written in lowercase)

## Git causeway

The causeway through which code enters a crannog and the agent's results
(commits, branches) leave it. Every crannog has one.

## Managed instructions block

The block between `<!-- crannog:begin -->` and `<!-- crannog:end -->` in each
agent's global instruction file in the agent home. It explains the crannog
to the agent; crannog replaces it on every build.

- **Not to be confused with:** the rest of the instruction file, which is
  the user's and which crannog leaves alone
- **In code:** `instructions.ManagedBlock`
- **In earlier ADRs:** marked block, managed block (ADR 0013, 0015, 0029)

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

## Readiness marker

`/tmp/ready` in a crannog, holding the boot id of the current start once the
image's entrypoint has prepared the crannog. crannog waits for it before it
runs a session.

- **In code:** `readyMarker`

## Repository

The user's git repository on the host from which crannogs are built. All of
its git worktrees count as one repository. Its configuration lives in
`.crannog/`; crannog keeps its state about it in
`~/.crannog/repositories/<id>/`.

- **Not to be confused with:** the copy of the repository inside a crannog
  (on the crannog's volume)
- **In code:** `Repository`

## Session

A process the user starts in a crannog with `crannog checkout`: usually an
interactive agent, also a shell or a non-interactive command. A crannog runs
while it has sessions. What a session runs by default is configured in
`.crannog/config.yml`.

- **Not to be confused with:** an agent's own sessions (conversations it
  stores, e.g. for resuming)

## Setup

The content of a `.crannog/` directory: `config.yml`, `iron-proxy.yaml` and
`container/`, from a working tree, a commit or a preset. A crannog uses one
setup at a time.

- **Not to be confused with:** preset, a template a setup can come from
- **In code:** `setup.Setup`

## Setup approval

The user's agreement to use a setup, i.e. a content of `.crannog/`, in
crannogs, given after seeing a diff. Each content is approved once; crannog
keeps approved setups in `approved/<sha256>/`. `--yes` never skips a setup
approval. Short: approval.

- **Not to be confused with:** confirmation; an agent's own approvals or
  permission prompts
- **In code:** `ui.UI.AskSetupApproval`

## Setup hash

The sha256 over a setup's files: paths, contents and executable bits. It
identifies approved setups. The image name uses the same hash computed over
`container/` alone (`crannog-<hash12>`).

- **In code:** `Setup.Hash`
- **In earlier ADRs:** content hash (ADR 0019)

## Start branch

The branch of the user's repository a crannog is built from and identified
by. A repository has at most one crannog per start branch. It is checked out
in the crannog; the agent may create further branches there.

- **In code:** `StartBranch`

## User

The human who sets up crannogs and controls their causeways.
