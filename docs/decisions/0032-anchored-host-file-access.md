# 0032. Anchored host file access

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

[ADR 0031](0031-crannog-writable-paths.md) confined host access to the
directories a guest can write to. A second review found destructive
operations elsewhere that followed paths out of crannog's state: a
symlinked crannog directory let a cleanup delete the contents of an
unrelated directory, a symlinked `sessions/` let self-repair delete files
in it, and a remote URL `crannog::../../x` led self-repair outside
`~/.crannog`. Fixing each place on its own leaves the next one open, and
every review has to search all file access again.

## Decision

Every file system access of crannog's code goes through an `os.Root` opened
on one of three **anchors**:

- **`~/.crannog`**, opened once per command (package `internal/home`).
- **`.crannog/` in a worktree.** The worktree's root is opened only to
  read, replace or check `.crannog/` and its temporary sibling
  `.crannog.tmp-*` (packages `internal/setup` and `internal/preset`).
- **Temporary directories crannog creates** in the user's temporary
  directory (`$TMPDIR` on macOS), named `crannog-<purpose>-*`. They are
  created and removed through a root on the temporary directory, opened
  once. macOS removes leftovers after a crash.

The rules:

- Package `internal/hostfs` opens anchors. Everything else works with
  paths relative to a root.
- No directory on the way from an anchor to a crannog's state may be a
  symlink, even one that stays within the anchor (`hostfs.OpenDir`). Each
  opened directory must be the one checked, so a replacement in between
  is refused.
  Directories a guest can write to get their own root (ADR 0031).
- Names read back from elsewhere are validated before they name a
  directory: branch slugs from remote URLs and directory entries, setup
  hashes. Session pruning only touches files named like a process id.
- The only access outside the anchors: checking that approved mount
  sources exist. The runtime mounts them anyway.
- Tests follow the same rules; they anchor in their temporary directory
  (package `internal/testfs`).
- A unit test parses all Go files and fails on file access by path
  (`os`, `syscall`, `ioutil`, `filepath.Walk*`, renameio without
  `WithRoot`, testify's file assertions) and on anchors opened outside the
  allowed packages. Exceptions are listed in it with their reason.

## Alternatives

- **Fixing the reported places only:** leaves the next one to the next
  review.
- **`~/.crannog/tmp` for temporary directories:** leftovers of a crash
  would stay forever; `$TMPDIR` is private to the user on macOS and cleaned
  by the system.
- **A fixed `/tmp/crannog`:** `/tmp` is shared; another user could create
  it first, e.g. as a symlink.
- **Allowing symlinks within `~/.crannog`:** a symlinked crannog directory
  could still make a cleanup delete another crannog's state.

## Consequences

- A symlink in crannog's state is an error instead of being followed. A
  user who wants `~/.crannog` elsewhere can still make `~/.crannog` itself
  a symlink: the anchor is opened through it.
- A remote whose URL is `crannog::` followed by something other than a
  branch slug is not crannog's; self-repair leaves it alone.
- Runtime objects (containers, volumes, networks) are not covered; their
  names are global and need their own ownership check.
- What git or the container CLI write is not covered; crannog only passes
  them paths it derived itself.
