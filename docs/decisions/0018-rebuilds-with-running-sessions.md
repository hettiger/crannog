# 0018. Rebuilds with running sessions

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0011](0011-repository-configuration.md) approves each content of
`.crannog/` once, whichever branch it comes from.
[ADR 0016](0016-cli-commands.md) has `checkout` and `apply` compare
`.crannog/` with the copy the crannog uses, show a diff and ask for approval
if it differs, and only warn that running sessions end when it rebuilds.

Read together, it is unclear whether content that was already approved
elsewhere, e.g. merged from another branch, needs approval again. If it does
not, a rebuild ends running sessions without asking.

## Decision

- **Approved once, approved always:** content of `.crannog/` that was
  approved before is never approved again (ADR 0011). Content that was not
  is shown as a diff and needs an approval, as before.
- **No running session:** if the content was approved before, crannog prints
  the source of `.crannog/` and rebuilds the crannog or reloads its proxy
  without asking.
- **Running sessions:** if a rebuild would end running sessions, crannog
  shows the diff to the crannog's copy and asks for confirmation. This is a
  confirmation, not an approval: `--yes` skips it
  ([ADR 0016](0016-cli-commands.md)). It replaces ADR 0016's warning that
  running sessions end.
- **One question:** if an approval is needed as well, the approval mentions
  that running sessions end. In `checkout`, the question replaces the
  confirmation for starting a session while another one runs.
- A change to `iron-proxy.yaml` only reloads the proxy, ends no session and
  needs no confirmation.

## Alternatives

- **Approving every difference to the crannog's copy:** asks again for
  content the user has already seen.
- **Only warning before a rebuild (ADR 0016):** sessions in other terminals
  end without the user agreeing.
- **Treating the question as an approval:** scripts could not rebuild a
  crannog with running sessions even with `--yes`, although nothing new is
  used.

## Consequences

- Merging an approved setup between branches needs no second approval.
- Running sessions only end after the user agreed to it or passed `--yes`.
- `checkout` asks at most one question before the build.
