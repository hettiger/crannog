# crannog

crannog lets coding agents work autonomously in sandboxes.

Like its namesake, an island dwelling in a lake, a crannog is isolated from
its surroundings. Each crannog runs in its own lightweight VM. The only ways
in and out are causeways you control:

- **git:** code enters through plain git remotes, and the agent's commits
  come back the same way. Nothing else of your repository or your machine
  is inside.
- **a network proxy:** the crannog reaches only the domains you allow.
- **mounts**, optionally, for what you decide to share.

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
```

This installs `crannog` and the git remote helper `git-remote-crannog`;
both must be in `PATH`. From a checkout, `make build` builds both into
`dist/`.

## Usage

In any git repository:

```sh
crannog checkout                      # session in the crannog of the checked-out branch
crannog checkout feature/x -- zsh     # a shell in the crannog of feature/x
crannog checkout -- claude -p "Fix the failing test"
```

The first `checkout` builds the crannog, which takes a few minutes. When
the last session ends, the crannog stops; the next session starts it again.

While a session runs, the crannog is a git remote of your repository:

```sh
git fetch crannog-<slug>              # the agent's branches
git push crannog-<slug> main          # your new commits into the crannog
```

| Command | Does |
| ------- | ---- |
| `crannog checkout [branch] [-- command...]` | Opens a session in the crannog of a branch |
| `crannog apply [branch]` | Applies changes to `.crannog/` without opening a session |
| `crannog destroy [branch]` | Destroys the crannog after showing what would be lost |
| `crannog list` | Lists the repository's crannogs |
| `crannog preset publish [name]` | Writes a preset to `.crannog/` |
| `crannog preset list` | Lists all presets |

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
