# Spike: agents in the default image

Evidence for [ADR 0013](../0013-agents-default-image.md). Rerun it after
updating Apple Containers or when an agent changes how it stores its state.

`container/` is a draft of crannog's default image. `spike.sh` runs an
iron-proxy and two crannogs with the agent homes relocated via the agents'
environment variables into one shared mount, Codex and OpenCode state on a
`/state` volume, and the repository at `/work/<host path>`. The proxy runs in
warn mode and lists the hosts the agents reach beyond a base allowlist.

## What it checks

1. **Image:** builds with Claude Code, Codex, OpenCode, Pi and Node.js.
2. **Crannogs:** separate `/work` and `/state` volumes owned by the agent
   user, repository at `/work/<host path>`, CA trusted by curl and Node
   (`NODE_USE_ENV_PROXY`), CA key not in the crannog.
3. **Agents:** all four run and are in `PATH` of a zsh login shell; their
   files land in the shared agent home, Codex's SQLite databases and
   OpenCode's database in `/state`; files written elsewhere are listed.
4. **Updates:** `claude update` and the npm registry work via the proxy.
5. **Manual (`--manual`):** logins in one crannog, reuse in the other, Claude
   Code calling OpenCode, OpenCode in both crannogs at once.

## Running

Requires Apple Silicon, macOS 26, the `container` CLI and internet access.

```bash
./spike.sh            # automated checks, cleans up afterwards
./spike.sh --manual   # keeps everything running, prints the manual steps
```

## Result

2026-11-01, `container` 1.6.1, Claude Code 2.1.316, Codex 0.166.0,
OpenCode v2.2.5, Pi 1.4.0: **28 passed, 0 failed.**

- Node's `fetch` ignores `HTTPS_PROXY` unless `NODE_USE_ENV_PROXY=1` is set.
- OpenCode's installer puts the binary in `~/.opencode/bin`, which a login
  shell does not have in `PATH`; the image links it into `~/.local/bin`.
- OpenCode writes snapshots and logs to `~/.local/share/opencode` and
  `~/.local/state/opencode`, and a `service.json` holding only a password into
  its config directory.
- Manual: Claude Code, Codex (device code) and Pi logins in one crannog worked
  in the other without logging in again. OpenCode's ChatGPT login (Codex
  device code) stayed in its crannog; Anthropic models worked through Claude
  Code. Claude Code ran OpenCode for a review. OpenCode ran in both crannogs
  at once.
- Codex warned that bubblewrap was missing and used its bundled copy. Pi
  downloaded ripgrep and fd from GitHub. Both led to adding these packages to
  the image afterwards; that change was not rerun.
- Hosts beyond the base allowlist: `pi.dev`, `opencode.ai`,
  `models.opencode.ai`, `github.com`, `release-assets.githubusercontent.com`,
  `http-intake.logs.us5.datadoghq.com`.
