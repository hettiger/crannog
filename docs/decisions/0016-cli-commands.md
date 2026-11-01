# 0016. CLI commands

- **Status:** Accepted, partially superseded by [0018](0018-rebuilds-with-running-sessions.md) (approval and warning on rebuild)
- **Date:** 2026-11-01

## Context

The decisions so far ([ADR 0010](0010-git-causeway.md) to
[ADR 0015](0015-presets.md)) need commands for starting sessions, applying
configuration changes, destroying crannogs and handling presets. The command
set should stay small: what crannog can decide from the state, it does
without a command.

## Decision

- **Style:** Cobra subcommands, nested only where several commands belong
  together (`crannog preset list`).
- **Commands:**

  | Command | Does |
  | ------- | ---- |
  | `crannog checkout [branch] [-- command...]` | Opens a session in the crannog of `branch` |
  | `crannog apply [branch]` | Applies changes to `.crannog/` without opening a session |
  | `crannog destroy [branch]` | Destroys the crannog |
  | `crannog list` | Lists the repository's crannogs: start branch, status, sessions |
  | `crannog preset publish [name]` | Writes a preset to `.crannog/` |
  | `crannog preset add [name]` | Saves the repository's `.crannog/` as a user preset |
  | `crannog preset list` | Lists all presets, bundled and user |
  | `crannog preset inspect [name]` | Prints the path of a user preset |
  | `crannog preset remove [name]` | Removes a user preset |

  `[branch]` defaults to the checked-out branch.
- **`checkout`** builds the crannog if it does not exist, starts it if it is
  stopped, and opens a session running `session.command`
  ([ADR 0013](0013-agents-default-image.md)) or the command after `--`, e.g.
  `crannog checkout feature/x -- zsh`. The command's exit code becomes
  crannog's exit code.
- **Changes to `.crannog/`:** `checkout` and `apply` compare `.crannog/` with
  the copy the crannog uses. If it differs, crannog shows the diff and asks
  for approval ([ADR 0011](0011-repository-configuration.md)); declining
  aborts. If only `iron-proxy.yaml` changed, crannog reloads the proxy and
  running sessions continue with the new rules. Otherwise it rebuilds the
  crannog and warns that running sessions end.
- **`destroy`** shows what would be lost and asks for confirmation
  ([ADR 0010](0010-git-causeway.md)).
- **`preset publish`** asks before replacing an existing `.crannog/`.
- **Interaction:** crannog allocates a terminal and asks questions only if
  stdin and stdout are terminals. Otherwise, a needed approval is an error,
  a confirmation is an error unless `--yes` is given, and the warning about
  a second session is printed without asking. `--yes` skips confirmations,
  never approvals. Commands that take a preset name show a picker if it is
  omitted.
- **Not commands:** building, rebuilding and stopping a crannog (implicit),
  running a non-interactive command (`checkout -- ...`), editing the
  allowlist and opening the proxy (`iron-proxy.yaml`, e.g. `warn: true`, then
  `apply`).

## Alternatives

- **`preset:publish` style names:** work against Cobra's help and
  completion.
- **Separate build and rebuild commands:** `checkout` and `apply` know when
  either is needed.
- **A separate `exec` command for scripts:** needs the same rules as
  `checkout`.
- **Commands for domains or opening the proxy:** `iron-proxy.yaml` can do
  more than a domain list, and changing it from a command would mean editing
  iron-proxy's format.
- **Entering the crannog with its old configuration after declining an
  approval:** hides that the configuration on the host differs from the
  crannog's.

## Consequences

- Day-to-day use is `crannog checkout` and `git fetch`.
- Scripts and loops can run agents non-interactively through `checkout`.
- Opening the proxy changes a file in the repository that must not be
  committed by accident; the next approval diff shows it.
- Cleanup beyond `destroy` is not covered yet.
