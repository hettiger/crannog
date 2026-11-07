# 0033. Conservative cleanup and replacement

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Anchored access ([ADR 0032](0032-anchored-host-file-access.md)) confines
filesystem paths, but does not establish which Git refs a subprocess may
delete, preserve old content during a failed replacement, or keep a lock
valid after its file is removed. A follow-up review found errors in these
operations.

## Decision

- Remove a matching crannog remote only when its sole fetch mapping is the
  generated `+refs/heads/*:refs/remotes/<remote>/*`. Refuse missing or changed
  mappings instead of letting `git remote remove` delete unrelated refs.
- Each destroy loss check uses a fresh, initially empty temporary ref
  namespace. Fetch only branches, without tag following, pruning or automatic
  maintenance; clean up only that invocation's namespace, without
  dereferencing symbolic refs. Existing refs from earlier checks stay.
- Initial repository transfer uses the derived crannog URL directly, so a
  remote's separate `pushurl` cannot redirect it.
- After acquiring a file lock, verify that its pathname still identifies
  the locked file. If destroy removed or replaced it while another command
  waited, that command fails and can be retried. Keep the existing lock
  locations and lifecycle from ADR 0025.
- Publishing a preset moves the old `.crannog` into the temporary sibling
  before installing the new setup. On installation failure, restore it. If
  restoring also fails, keep the backup and report its recovery path.
- The smoke test passes paths as arguments, never interpolates them into
  shell source, and uses `crannog destroy` for host-state cleanup. It keeps
  approved setups rather than recursively deleting repository state by path.

## Alternatives

- Trust a remote's name and URL alone: its fetch mappings and push URL can
  differ from the generated configuration.
- Reuse a fixed temporary ref namespace: cleanup can delete preexisting
  refs, including after a failed or declined destroy.
- Delete the old setup before renaming the new one: a failed rename loses
  the original content.
- Keep using a lock on an unlinked file: another process can hold a lock
  on a newly created file at the same path concurrently.

## Consequences

Unusual remote configurations and replaced locks produce errors instead
of proceeding. Preset directory replacement is recoverable, not atomic:
a crash between moves can leave the backup at `.crannog.tmp-*/old`.
Atomic individual file writes still use `renameio.WithRoot`.
Runtime object ownership remains outside this filesystem rule.
