# 0031. Crannog-writable paths and reviewable configuration

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

crannog's host code reads and writes files in directories that a guest can
write to: the agent homes in `~/.crannog/agents/` (mounted writable into
every crannog, [ADR 0008](0008-agent-home.md)) and a crannog's
`proxy/` directory (written by `generate-ca` in a container of the proxy
image, which the setup chooses). A guest can place symlinks to host paths
there or replace a directory by a symlink while crannog works on it.
Checking with `Lstat` first does not help against such a race. A security
review reproduced both: a host file read into an agent home, a host file
overwritten, the mode of a host file changed.

The same review showed that the approval diff can hide configuration:
YAML parsers accept UTF-16, which `git diff` shows as `Binary files
differ`, and control or format characters can hide or reorder text in the
terminal.

## Decision

- **Host access to crannog-writable directories** goes through an
  `os.Root` opened on a directory the guest cannot replace (the mount root
  or a directory below it, opened through the parent's root). Symlinks are
  followed only within the root. Files are read only if they are regular
  files (opened with `O_NONBLOCK`, checked on the open file) and up to a
  size limit. Writes stay atomic with renameio's `WithRoot`.
  Package `internal/untrusted` holds the read side.
- **The proxy mounts `proxy/` read-only.** Only the one-off `generate-ca`
  container writes it. crannog checks that `ca.crt` is exactly one
  PEM-encoded CA certificate before it copies it to the CA directory, and
  sets the mode of `ca.key` on the open file.
- **`config.yml` and `iron-proxy.yaml` must be reviewable:** valid UTF-8
  without control characters other than tab, `\n` and `\r\n`, and without
  Unicode format characters (byte order mark, bidi overrides, zero-width
  characters). Otherwise the setup is invalid and crannog stops before any
  approval. Files in `container/` may be binary.

## Alternatives

- **`Lstat` before each access, or a host-side lock:** races with the guest,
  which does not take host locks.
- **A hand-written atomic write within the root:** renameio supports
  `os.Root` already.
- **Writing the managed instructions block from inside the crannog:**
  avoids host access to the agent homes, but needs a running crannog,
  while `apply` builds stopped ones ([ADR 0029](0029-poc-lifecycle-details.md)).
- **A decoded, semantic diff of the configuration:** more work and a
  second view to trust; requiring plain text keeps the diff faithful.

## Consequences

- Symlinked agent homes and instruction files stay errors; one swapped in
  during a write cannot lead out of its agent home.
- Instruction files larger than 1 MiB are an error.
- Proxy images must not need to write their configuration directory while
  they run.
- Configuration files with a byte order mark, CR-only line breaks or
  emoji with zero-width joiners are refused.
