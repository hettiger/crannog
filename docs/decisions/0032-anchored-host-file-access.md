# 0032. Anchored host file access

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

[ADR 0031](0031-crannog-writable-paths.md) confined host access to the
directories a guest can write to. A second review found destructive
operations elsewhere that followed paths out of crannog's state, e.g. a
symlinked crannog directory let a cleanup delete an unrelated directory.
Fixing each place on its own leaves the next one open, and every review has
to search all file access again.

## Decision

All host file access of crannog goes through an `os.Root` opened on one of
three **anchors**:

- `~/.crannog`
- `.crannog/` in a worktree (and its temporary sibling while it is
  replaced)
- temporary directories crannog creates in `$TMPDIR`, named
  `crannog-<purpose>-*`

Below an anchor, symlinks are not followed; a symlink in crannog's state is
an error. Names from outside (branch slugs, setup hashes) are validated
before they name a directory. A unit test fails on file access by path, in
code and tests alike.

## Alternatives

- **Fixing the reported places only:** leaves the next one to the next
  review.
- **`~/.crannog/tmp` for temporary directories:** leftovers of a crash
  would stay forever; macOS cleans `$TMPDIR`.
- **A fixed `/tmp/crannog`:** `/tmp` is shared; another user could create
  it first.
- **Allowing symlinks within `~/.crannog`:** a symlinked crannog directory
  could still make a cleanup delete another crannog's state.

## Consequences

- `~/.crannog` itself may still be a symlink: the anchor is opened through
  it.
- Approved mount sources are outside the anchors; crannog only checks that
  they exist.
- Runtime objects (containers, volumes, networks) are not covered; their
  names are global and need their own ownership check.
