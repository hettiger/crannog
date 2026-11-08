# 0031. Host file access

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

crannog writes and deletes files on the host: in `~/.crannog`, in a
worktree's `.crannog/` and in temporary directories. A guest can write to
some of them: the agent homes in `~/.crannog/agents/` (mounted writable into
every crannog, [ADR 0008](0008-agent-home.md)) and a crannog's `proxy/`
directory (written by `generate-ca` in a container of the proxy image,
which the setup chooses). A guest can place symlinks to host paths there,
or swap a directory for a symlink while crannog works on it, and so make
crannog read, overwrite or chmod host files. In crannog's own state, a
symlink or a crafted name can likewise make a cleanup delete unrelated
files. Securing each operation on its own leaves the next one open.

## Decision

- All host file access goes through an `os.Root` opened on one of three
  **anchors**:
  - `~/.crannog`
  - `.crannog/` in a worktree (and its temporary sibling while it is
    replaced)
  - temporary directories crannog creates in `$TMPDIR`, named
    `crannog-<purpose>-*`
- Below an anchor, a symlink in crannog's state is an error, never
  followed. Names from outside (branch slugs, setup hashes) are validated
  before they name a directory.
- Directories a guest can write to are untrusted: crannog never follows a
  path out of them and reads only regular files of limited size.
- The proxy mounts `proxy/` read-only. Only the one-off `generate-ca`
  container writes it.
- A unit test fails on file access by path, in code and tests alike.

## Alternatives

- **Checking with `Lstat` before each access:** races with the guest.
- **Securing each operation on its own:** the next one is easy to miss.
- **Allowing symlinks within `~/.crannog`:** a symlinked crannog directory
  could still make a cleanup delete another crannog's state.
- **Temporary directories in `~/.crannog/tmp` or a fixed `/tmp/crannog`:**
  leftovers of a crash would stay forever, and in the shared `/tmp`
  another user could create the directory first. macOS cleans the user's
  `$TMPDIR`.
- **Writing the managed instructions block from inside the crannog:**
  needs a running crannog, while `apply` builds stopped ones
  ([ADR 0029](0029-poc-lifecycle-details.md)).
- **Checking the generated CA certificate:** the proxy image is part of the
  approved setup; a malicious one generates a valid certificate anyway.

## Consequences

- Symlinked agent homes and instruction files are errors. `~/.crannog`
  itself may be a symlink: the anchor is opened through it.
- Proxy images must not need to write their configuration directory while
  they run.
- Approved mount sources are outside the anchors; crannog only checks that
  they exist.
- Runtime objects (containers, volumes, networks) are not covered; their
  names are global and need their own ownership check.
