# apply

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0011](../decisions/0011-repository-configuration.md), [0015](../decisions/0015-presets.md), [0016](../decisions/0016-cli-commands.md), [0018](../decisions/0018-rebuilds-with-running-sessions.md), [0025](../decisions/0025-session-locks-and-lifecycle.md), [0029](../decisions/0029-poc-lifecycle-details.md), [0031](../decisions/0031-crannog-writable-paths.md)

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
