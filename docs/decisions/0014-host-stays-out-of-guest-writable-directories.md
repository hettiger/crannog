# 0014. Host stays out of guest-writable directories

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

Guests can write to some host directories: the agent homes in
`~/.crannog/agents/` and the caches in `~/.crannog/cache/` are mounted
writable into every crannog ([ADR 0011](0011-agent-home.md),
[ADR 0012](0012-shared-package-caches.md)). A guest can place symlinks to
host paths there, or swap a directory for a symlink while crannog works on
it, and so make crannog read, overwrite or chmod host files. Host code that
works in such a directory has to be safe against a concurrent attacker at
every access, and every review has to check that again.

## Decision

- crannog's host code never reads, writes or deletes anything in a
  directory a guest can write to. It only creates such a directory, in a
  parent no guest can write to.
- crannog generates the CA on the host and writes it into the crannog's
  `proxy/` directory, which only the host writes. The proxy mounts `proxy/`
  read-only.
- What crannog keeps in guest-writable directories, the crannog writes
  itself, through `container exec` as the agent user. Only content passes
  between host and crannog. This is how the managed instructions block gets
  into the agent homes ([ADR 0029](0029-managed-instructions-block.md)).

## Alternatives

- **Safe access to guest-writable directories** (an `os.Root` per
  directory, checks against swapped directories, regular files of limited
  size only): works, but every access has to be reviewed against a
  concurrent attacker.

## Consequences

- A review of host file access checks two things: which host directories
  are mounted writable, and that host code does not access them.
- Proxy images must not need to write their configuration directory.
