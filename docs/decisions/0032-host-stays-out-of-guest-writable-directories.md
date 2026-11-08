# 0032. Host stays out of guest-writable directories

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

Guests can write to some host directories: the agent homes in
`~/.crannog/agents/` and the caches in `~/.crannog/cache/` are mounted
writable into every crannog ([ADR 0008](0008-agent-home.md),
[ADR 0009](0009-shared-package-caches.md)). A guest can place symlinks to
host paths there, or swap a directory for a symlink while crannog works on
it, and so make crannog read, overwrite or chmod host files. Host code that
works in such a directory has to be safe against a concurrent attacker at
every access, and every review has to check that again.

## Decision

- crannog's host code never reads, writes or deletes anything in a
  directory a guest can write to. It only creates such a directory, in a
  parent no guest can write to.
- crannog generates the CA on the host and writes it into `proxy/`, which
  only the host writes. The proxy mounts `proxy/` read-only.
- The crannog writes the managed instructions block into the agent homes
  itself: before each session, crannog reads and writes the instruction
  files with `container exec` as the agent user. Only content passes
  between host and crannog.

## Alternatives

- **Safe access to guest-writable directories** (an `os.Root` per
  directory, checks against swapped directories, regular files of limited
  size only): works, but every access has to be reviewed against a
  concurrent attacker.
- **Generating the CA with the proxy image** (`iron-proxy generate-ca`
  into `proxy/`): the image writes into a directory the host reads.
- **Writing the managed instructions block on every build:** needs host
  access to the agent homes, and a crannog stays on an outdated block until
  its next rebuild.

## Consequences

- A review of host file access checks two things: which host directories
  are mounted writable, and that host code does not access them.
- The managed instructions block is up to date after every session start.
  This supersedes "on every build" in [ADR 0013](0013-agents-default-image.md)
  and the host creating `~/.crannog/agents/<agent>/` in
  [ADR 0029](0029-poc-lifecycle-details.md); the crannog creates them.
- Without a writable mount of the agent homes, no block is written.
- Each session start runs one more `container exec` per agent, two if the
  block changes.
- The CA no longer depends on the proxy image. Proxy images must read a
  PEM certificate and a PKCS #8 key and must not need to write their
  configuration directory.
