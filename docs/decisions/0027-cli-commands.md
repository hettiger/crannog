# 0027. CLI commands

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog needs commands for starting sessions, applying setup changes,
destroying crannogs and handling presets. The command set should stay
small: what crannog can decide from the state, it does without a command.

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

- **`[branch]`** defaults to the checked-out branch and must be a local
  branch (`refs/heads/<branch>`). `destroy` also accepts the start branch of
  a crannog whose branch was deleted.
- **`checkout`** builds the crannog if it does not exist, starts it if it is
  stopped, and opens a session running `session.command` or the command
  after `--`, e.g. `crannog checkout feature/x -- zsh`. crannog quotes each
  argument after `--` for the shell, so the login shell runs exactly these
  arguments: `-- claude -p "say hi"` passes `say hi` as one argument. The
  session's exit code becomes crannog's exit code.
- **Setup changes:** `checkout` and `apply` compare the setup with the
  crannog's and ask for a setup approval
  ([ADR 0024](0024-setup-approval.md)) or a confirmation before a rebuild
  ([ADR 0022](0022-rebuilds.md)); declining aborts.
- **`destroy`** shows what would be lost and asks for confirmation
  ([ADR 0015](0015-git-causeway.md)), also saying how many running sessions
  end.
- **`preset publish`** asks before replacing an existing `.crannog/`.
- **Interaction:** crannog allocates a terminal and asks questions only if
  stdin and stdout are terminals. Otherwise, a needed approval is an error,
  a confirmation is an error unless `--yes` is given, and the warning about
  a second session is printed without asking. `--yes` answers
  confirmations, never approvals. Commands that take a preset name show a
  picker if it is omitted.
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
- **Joining the arguments after `--` with spaces, like ssh:** `-p "say hi"`
  would arrive as two arguments.
- **Remote-tracking refs as `[branch]`:** unclear what the start branch is.
- **Commands for domains or opening the proxy:** `iron-proxy.yaml` can do
  more than a domain list, and changing it from a command would mean editing
  iron-proxy's format.
- **Entering the crannog with its old setup after declining an approval:**
  hides that the setup on the host differs from the crannog's.

## Consequences

- Day-to-day use is `crannog checkout` and `git fetch`.
- Scripts and loops can run agents non-interactively through `checkout`;
  shell syntax needs an explicit shell, e.g.
  `crannog checkout -- sh -c 'echo $HOME'`.
- Opening the proxy changes a file in the repository that must not be
  committed by accident; the next approval diff shows it.
- Cleanup beyond `destroy` is not covered yet.
