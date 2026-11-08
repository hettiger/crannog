# 0013. Host file access

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

crannog writes and deletes files on the host: in `~/.crannog`, in a
worktree's `.crannog/` and in temporary directories. A symlink or a crafted
name in crannog's state can make an operation such as a cleanup delete or
overwrite unrelated files. Securing each operation on its own leaves the
next one open. Directories a guest can write to are covered by
[ADR 0014](0014-host-stays-out-of-guest-writable-directories.md).

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
- A unit test fails on file access by path, in code and tests alike.

## Alternatives

- **Securing each operation on its own:** the next one is easy to miss.
- **Allowing symlinks within `~/.crannog`:** a symlinked crannog directory
  could still make a cleanup delete another crannog's state.
- **Temporary directories in `~/.crannog/tmp` or a fixed `/tmp/crannog`:**
  leftovers of a crash would stay forever, and in the shared `/tmp`
  another user could create the directory first. macOS cleans the user's
  `$TMPDIR`.

## Consequences

- `~/.crannog` itself may be a symlink: the anchor is opened through it.
- Approved mount sources are outside the anchors; crannog only checks that
  they exist.
- Runtime objects (containers, volumes, networks) are not covered; their
  names are global and need their own ownership check.
