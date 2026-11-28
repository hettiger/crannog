# 0027. CLI commands

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog needs commands for starting sessions, applying setup changes,
destroying crannogs, handling presets and reaching a crannog from tools on
the host. The command set should stay small: what crannog can decide from
the state, it does without a command.

## Decision

- **Style:** Cobra subcommands, nested only where several commands belong
  together (`crannog preset list`).
- **Commands:**

  | Command | Does |
  | ------- | ---- |
  | `crannog checkout [branch] [-- command...]` | Opens a session in the crannog of `branch` |
  | `crannog checkout -d [branch] [-- command...]` | Starts a detached session and returns |
  | `crannog apply [branch]` | Applies changes to `.crannog/` without opening a session |
  | `crannog stop [branch]` | Ends the sessions of the crannog and stops it |
  | `crannog destroy [branch]` | Destroys the crannog |
  | `crannog list` | Lists the repository's crannogs: start branch, status, sessions |
  | `crannog ssh [branch] [-- command...]` | Opens a shell over SSH in the running crannog of `branch`, or runs a command there |
  | `crannog ssh [branch] --host` / `--path` | Prints the crannog's SSH host, or `<host>:<path>` |
  | `crannog open [branch]` | Opens the running crannog of `branch` in the browser |
  | `crannog open [branch] --zed` | Opens the repository in the running crannog of `branch` in Zed |
  | `crannog preset publish [name]` | Writes a preset to `.crannog/` |
  | `crannog preset add [name]` | Saves the repository's `.crannog/` as a user preset |
  | `crannog preset list` | Lists all presets, bundled and user |
  | `crannog preset inspect [name]` | Prints the path of a user preset |
  | `crannog preset remove [name]` | Removes a user preset |

- **`[branch]`** defaults to the checked-out branch and must be a local
  branch (`refs/heads/<branch>`). `stop` and `destroy` also accept the start
  branch of a crannog whose branch was deleted.
- **`checkout`** builds the crannog if it does not exist, starts it if it is
  stopped, and opens a session running `session.command` or the command
  after `--`, e.g. `crannog checkout feature/x -- zsh`. crannog quotes each
  argument after `--` for the shell, so the login shell runs exactly these
  arguments: `-- claude -p "say hi"` passes `say hi` as one argument. The
  session's exit code becomes crannog's exit code.
- **`checkout -d`** (`--detach`), like `docker compose up -d`, does the
  same, but starts a detached session
  ([ADR 0019](0019-sessions-and-auto-stop.md)) and returns. Without a
  command after `--` the session only waits.
- **`ssh`** runs the system `ssh` with the user's SSH configuration, like
  every tool on the host ([ADR 0032](0032-ssh-causeway.md)). For the user
  it works like a session: the agent's login shell in the repository, or
  the command after `--`, quoted like `checkout`'s; ssh's exit code becomes
  crannog's. It is no session: it never starts the crannog and does not
  keep it running. `--host` and `--path` print the SSH host and
  `<host>:<path>` for other tools and scripts (scp, rsync, editors
  connected by hand); they only need the crannog to exist.
- **`open`** opens a running crannog on the host, only through URLs that
  macOS hands to an app (`/usr/bin/open`): in the browser at the crannog's
  address on its internal network, which the Mac reaches (`https://`, or
  `http://` with `--insecure`, `--port` for another port), or in Zed with
  `--zed` (`zed://ssh/…`, needs the `Include` in `~/.ssh/config`). Before
  the browser opens, crannog checks that something accepts a TCP
  connection there and fails otherwise; the server is the setup's
  business. Tools without such a URL are connected by hand with
  `ssh --host`. There are no commands to close a tool: that is done in
  the tool.
- **`stop`** asks nothing. Stopping a crannog after its last session stays
  implicit.
- **Setup changes:** `checkout` and `apply` compare the setup with the
  crannog's and ask for a setup approval
  ([ADR 0024](0024-setup-approval.md)) or a confirmation before a rebuild
  ([ADR 0022](0022-rebuilds.md)); declining aborts.
- **`destroy`** asks for confirmation
  ([ADR 0015](0015-git-causeway.md)), also saying how many running sessions
  end.
- **`preset publish`** asks before replacing an existing `.crannog/`.
- **Completion:** Cobra's `completion` command. A command's first argument
  completes to what can work there: `checkout` to the local branches,
  `apply`, `stop`, `destroy`, `ssh` and `open` to the start branches
  of the repository's crannogs, running or not, `preset publish` to the
  preset names. Completion is fast and has no side effects beyond opening
  `~/.crannog` (which creates the empty directory): it never repairs,
  never prompts and never queries the runtime; an error offers nothing. After the first
  argument and after `--` it offers nothing, not even files.
- **Interaction:** crannog allocates a terminal and asks questions only if
  stdin and stdout are terminals. Otherwise, a needed approval is an error,
  a confirmation is an error unless `--yes` is given, and the warning about
  a second session is printed without asking. `--yes` answers
  confirmations, never approvals. Commands that take a preset name show a
  picker if it is omitted.
- **Not commands:** building and rebuilding a crannog (implicit), running a
  non-interactive command (`checkout -- ...`), editing the
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
- **Tools under `ssh` (`crannog ssh zed`):** would show that they need SSH,
  but `crannog ssh <branch>` could not reach a branch named like a tool.
- **A command group per tool (`crannog rex connect`):** one more top-level
  command for every tool.
- **`use <tool>` with a subcommand per tool (`use zed`, `use rex`,
  `use phpstorm`):** crannog runs in Seatbelt
  ([ADR 0030](0030-seatbelt.md)), so it can only hand URLs to apps. Rex
  has no such URL (`rex hosts add` from crannog did not show the host in
  Rex's window), and JetBrains' Gateway link opens only an IDE that is
  already installed in the crannog. Printing the host serves every tool.
- **Zed's command line tool `zed` for `open --zed`:** it starts Zed in a
  way that fails in any sandbox, Seatbelt too. `open --zed` opens Zed's
  URL with macOS's `open` instead.
- **Opening the browser without checking for a server:** the browser
  would show its own error page or wait, without saying why.
- **Completing only running crannogs for `stop`, `ssh` and `open`:** every
  Tab would ask the runtime; a stopped crannog is rare there, and the
  command reports it when it runs.
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
