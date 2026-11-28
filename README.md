# crannog

crannog lets coding agents work autonomously in sandboxes.

Like its namesake, an island dwelling in a lake, a crannog is isolated from
its surroundings. Each crannog runs in its own lightweight VM. The only ways
in and out are causeways you control:

- **git:** code enters through plain git remotes, and the agent's commits
  come back the same way. Nothing else of your repository or your machine
  is inside.
- **a network proxy:** the crannog reaches only the domains you allow,
  plus services on your Mac that listen on all interfaces (`0.0.0.0`).
- **mounts**, optionally, for what you decide to share.
- **SSH**, optionally, for editors and other tools on your Mac.

Claude Code, Codex, OpenCode and Pi come preinstalled, share their logins
across crannogs and can call each other. They run without permission
prompts: the crannog is the security boundary.

> **Status:** proof of concept. Expect rough edges and breaking changes.

## Requirements

- macOS 26 or later on Apple Silicon
- [Apple Containers](https://github.com/apple/container) (`container`
  CLI, `container system start` has run once)
- git

## Installation

```sh
go install github.com/hettiger/crannog/...@latest
ln -s "$(go env GOPATH)/bin/git-remote-crannog" /usr/local/bin/git-remote-crannog
```

This installs `crannog` and the git remote helper `git-remote-crannog`;
both must be in `PATH`. The symlink makes the helper visible to git clients
that do not load your shell's `PATH`, such as GUI apps. From a checkout,
`make build` builds both into `dist/`.

## Usage

In any git repository:

```sh
crannog checkout                      # session in the crannog of the checked-out branch
crannog checkout feature/x -- zsh     # a shell in the crannog of feature/x
crannog checkout -- claude -p "Fix the failing test"
crannog checkout -d                   # keep the crannog running in the background
crannog stop                          # end its sessions and stop it
```

The first `checkout` builds the crannog, which takes a few minutes. When
the last session ends, the crannog stops; the next session starts it again.
With `-d`, crannog starts a detached session, which only waits or runs the
command after `--`, and returns: the crannog keeps running in the
background.

While a session runs, the crannog is a git remote of your repository:

```sh
git fetch crannog-<slug>              # the agent's branches
git push crannog-<slug> main          # your new commits into the crannog
```

| Command | Does |
| ------- | ---- |
| `crannog checkout [branch] [-- command...]` | Opens a session in the crannog of a branch |
| `crannog checkout -d [branch] [-- command...]` | Starts a detached session and returns |
| `crannog apply [branch]` | Applies changes to `.crannog/` without opening a session |
| `crannog stop [branch]` | Ends the sessions of the crannog and stops it |
| `crannog destroy [branch]` | Destroys the crannog after a confirmation |
| `crannog list` | Lists the repository's crannogs |
| `crannog ssh [branch] [-- command...]` | Opens a shell over SSH in a running crannog |
| `crannog ssh [branch] --host` / `--path` | Prints the crannog's SSH host, or `<host>:<path>` for scp |
| `crannog open [branch]` | Opens a running crannog in the browser (`--insecure`, `--port`) |
| `crannog open [branch] --zed` | Opens a running crannog in Zed |
| `crannog preset publish [name]` | Writes a preset to `.crannog/` |
| `crannog preset list` | Lists all presets |

`crannog completion <shell>` prints a completion script for bash, zsh,
fish or PowerShell, which completes branches and preset names; see
`crannog completion --help`.

## Tools on your Mac

Editors with remote development (Zed, PhpStorm), terminal multiplexers and
`ssh` reach a running crannog over SSH, without a network port. Enable it
once by adding this line at the top of `~/.ssh/config`:

```
Include ~/.crannog/ssh/config
```

crannog never edits `~/.ssh` itself. Then open a shell with `crannog ssh`,
open the repository in Zed with `crannog open --zed`, or connect tools to
the host `crannog ssh --host` prints:

```sh
crannog checkout -d
crannog ssh
crannog open --zed
scp notes.txt "$(crannog ssh --path)/"
```

`crannog open` opens a server in the crannog in your browser, at the
crannog's address.

See [SSH causeway](docs/features/ssh.md) for Zed, JetBrains IDEs, Rex and
caveats, and [open](docs/features/open.md).

## Configuration

A repository configures its crannogs in `.crannog/`: `config.yml` (session
command, resources, proxy image, environment, mounts), `iron-proxy.yaml`
(the proxy's allowlist and settings) and `container/` (the image's build
context). Without `.crannog/`, crannog uses the bundled preset `default`;
`crannog preset publish default` writes it to the repository as a starting
point.

crannog only uses a setup you approved: when `.crannog/` changes, including
changes an agent committed, it shows the diff and asks before using it.

## Documentation

[`docs/`](docs/README.md) holds the glossary, the architecture decisions,
the features and the use cases.

## License

[MIT](LICENSE)
