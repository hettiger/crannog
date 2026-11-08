# apply

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0019](../decisions/0019-sessions-and-auto-stop.md), [0021](../decisions/0021-crannog-state-on-the-host.md), [0022](../decisions/0022-rebuilds.md), [0023](../decisions/0023-repository-configuration.md), [0024](../decisions/0024-setup-approval.md), [0027](../decisions/0027-cli-commands.md)

## Summary

Applies changes to `.crannog/` to the crannog of a start branch without
opening a session.

## Usage

```sh
crannog apply [branch] [--yes]
```

## Behavior

- Without a crannog for the branch: an error that points to
  `crannog checkout`.
- Reads `.crannog/` like [checkout](checkout.md) and compares it with the
  crannog's copy. Unchanged: prints that the crannog is up to date.
- Content not approved before needs an approval (diff to the crannog's
  copy, host variables it reads, whether running sessions end). Without a
  terminal this is an error, also with `--yes`.
- Only `iron-proxy.yaml` changed: writes the proxy's copy and reloads the
  proxy if it runs; running sessions continue. If the proxy rejects the
  file, the old one is restored.
- Otherwise: builds the new image first, then replaces proxy and crannog.
  With running sessions and approved content, asks for confirmation first
  (`--yes` answers it). The rebuilt crannog stays stopped until the next
  session.

After an interrupted rebuild, resolve its cause and retry apply with the setup
that triggered the rebuild. If it still requires a rebuild relative to the
recorded setup, apply recreates missing containers while preserving the work
and state volumes. An unchanged or proxy-only setup does not recreate them;
"up to date" describes the setup, not container health. Destroy deletes the
volumes and is not a data-preserving recovery step.
