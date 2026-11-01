# 0014. One crannog per start branch

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0012](0012-crannog-names-volumes-paths.md) gives each crannog a name and
allows several crannogs per start branch. Commands then need rules for
explicit names, derived names, conflicts and selection, which makes the CLI
hard to explain. A second crannog from the same branch is rarely needed and
is one `git branch` away.

[ADR 0010](0010-git-causeway.md) keeps a crannog's remote from build to
destroy. Users keep crannogs around they have not cleaned up yet, and each of
them adds a remote to the repository.

Every crannog is a VM that uses memory while it runs. A crannog left running
by mistake is easy to forget.

## Decision

- **Identity:** a repository has at most one crannog per start branch.
  Commands identify crannogs by their start branch (`[branch]`, defaulting to
  the checked-out branch). There are no crannog names in the CLI. This
  supersedes the name and selection rules of ADR 0012.
- **Slug:** crannog derives a slug from the start branch with the rule of ADR
  0012 (`feature/Foo_bar` → `feature-foo-bar`). It is used only internally:
  in the remote name, the runtime names and the host directory. If two
  branches lead to the same slug, crannog refuses with an error.
- **State on the host:**

  ```
  ~/.crannog/repositories/<id>/crannogs/<slug>/
    branch      # the start branch (the slug is not reversible)
    crannog     # hash of the approved copy this crannog uses
    lock        # see below
    proxy/
    ca/
  ```

- **Sessions:** several sessions can run in one crannog at the same time.
  They share the working tree and everything running in the crannog. Before
  starting a session while another one runs, crannog asks for confirmation.
- **Auto-stop:** when the last session ends, however it ends, crannog stops
  the crannog and its proxy. Background processes end with it. The next
  session starts the crannog again. There is no stop command.
- **Remote only while running:** crannog adds the remote `crannog-<slug>`
  when it starts a crannog and removes it when it stops it. If a remote of
  that name exists, crannog refuses to start. The remote helper never starts
  a crannog; a stopped crannog has no remote. This supersedes "added on
  build, removed on destroy" of ADR 0010. crannog never sets a branch's
  upstream; removing the remote also removes upstream settings pointing to
  it.
- **Self-repair:** on every call in a repository, crannog removes `crannog-*`
  remotes whose crannog is not running and stops running crannogs without a
  session, removing their remotes. A command that starts a crannog holds an
  exclusive `flock` on its `lock` file until the session runs; self-repair
  skips locked crannogs. The kernel releases the lock when the process ends,
  so it never stays behind.
- **Destroy** fetches directly via `crannog::<slug>`, starting the crannog if
  needed, before it removes anything.

## Alternatives

- **Names and several crannogs per branch (ADR 0012):** most of the CLI's
  rules exist only for this case.
- **Keeping crannogs running after the last session:** forgotten crannogs use
  memory until someone stops them.
- **Remote from build to destroy:** the number of remotes grows with the
  crannogs lying around. Fetching a stopped crannog would need the remote
  helper to start it.
- **Lock files without `flock`:** stay behind when crannog is killed.

## Consequences

- Users address crannogs like branches and never pick a name.
- A second crannog for the same branch needs a new branch.
- `git remote` lists exactly the running crannogs. Remote-tracking refs of a
  crannog disappear when it stops and return after the next fetch once it
  runs again; work must be fetched and merged while a session runs.
- Background services in a crannog only run while a session does.
- Loops that start a session per iteration start and stop the VM each time.
- If crannog is killed, the crannog keeps running until the next crannog call
  in the repository.
