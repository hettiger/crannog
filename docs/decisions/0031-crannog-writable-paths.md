# 0031. Crannog-writable paths and reviewable configuration

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

crannog's host code reads and writes files in directories that a guest can
write to: the agent homes in `~/.crannog/agents/` (mounted writable into
every crannog, [ADR 0008](0008-agent-home.md)) and a crannog's `proxy/`
directory (written by `generate-ca` in a container of the proxy image,
which the setup chooses). A guest can place symlinks to host paths there,
or swap a directory for a symlink while crannog works on it. A security
review used this to read, overwrite and chmod host files.

The same review showed that the approval diff can hide configuration:
YAML parsers accept UTF-16, which `git diff` shows as binary, and control
or format characters can hide or reorder text in the terminal.

## Decision

- **Guest-writable directories are untrusted.** The host never follows a
  path out of them: it opens them as an `os.Root` and reads only regular
  files of limited size.
- **The proxy mounts `proxy/` read-only.** Only the one-off `generate-ca`
  container writes it, and crannog checks the CA certificate before it
  uses it.
- **`config.yml` and `iron-proxy.yaml` must be plain text,** so the
  approval diff shows what is approved: UTF-8 without control characters
  other than tab and line breaks, and without Unicode format characters.
  Otherwise the setup is invalid. Files in `container/` may be binary.

## Alternatives

- **Checking with `Lstat` before each access:** races with the guest.
- **Writing the managed instructions block from inside the crannog:**
  needs a running crannog, while `apply` builds stopped ones
  ([ADR 0029](0029-poc-lifecycle-details.md)).
- **A decoded, semantic diff of the configuration:** a second view to
  trust; requiring plain text keeps the diff faithful.

## Consequences

- Symlinked agent homes and instruction files are errors.
- Proxy images must not need to write their configuration directory while
  they run.
- Configuration files with a byte order mark or emoji with zero-width
  joiners are refused.
