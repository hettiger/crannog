# 0029. Lifecycle details of the PoC

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

Implementing the PoC of `checkout`, `apply`, `destroy` and the git remote
helper ([ADR 0016](0016-cli-commands.md)) raised questions the earlier ADRs
leave open: how arguments after `--` reach the login shell, what a rebuild
replaces, what `apply` does with a stopped crannog, what happens when a
build or the check before `destroy` fails, which agents crannog knows, and
what `[branch]` may name.

## Decision

- **`checkout -- command...`:** crannog quotes each argument for the shell
  and joins them, so the login shell runs exactly these arguments:
  `crannog checkout -- claude -p "say hi"` passes `say hi` as one
  argument. Shell syntax needs an explicit shell, e.g.
  `crannog checkout -- sh -c 'echo $HOME'`.
- **Rebuild:** replaces both containers, the crannog and its proxy, so
  changes to `proxy.image`, `env`, `mounts` and `resources` take effect.
  Volumes, the network, the CA, the remote's name and the host directory
  stay.
- **`apply` never starts a crannog:** a rebuild creates the new containers
  stopped; the next session starts them. If only `iron-proxy.yaml` changed
  and the proxy is stopped, crannog only writes the proxy's copy.
- **Failed first build:** crannog removes everything that build created
  (containers, network, volumes, remote, host state except the lock); the
  next `checkout` starts over. The image stays.
- **`destroy` without a check:** if the crannog cannot be started or
  fetched, crannog says that it cannot check what would be lost and asks
  for confirmation; `--yes` answers it.
- **Supported agents:** crannog knows Claude Code (`claude`, `CLAUDE.md`),
  Codex (`codex`, `AGENTS.md`), OpenCode (`opencode`, `AGENTS.md`) and Pi
  (`pi`, `AGENTS.md`). It creates `~/.crannog/agents/<agent>/` for each and
  `~/.crannog/cache/`, and keeps the managed block in each instruction file.
  A mount source in `config.yml` that does not exist is an error.
- **`[branch]`** of `checkout` and `apply` must be a local branch
  (`refs/heads/<branch>`). `destroy` also accepts the start branch of a
  crannog whose branch was deleted.
- **E2E smoke test:** `e2e/smoke.sh` runs on a Mac with Apple Containers
  against the real `~/.crannog` and removes the state of its temporary
  repository afterwards. `e2e/` is a top-level directory next to the
  binaries ([ADR 0027](0027-repository-layout.md)).

## Alternatives

- **Joining the arguments with spaces, like ssh:** `-p "say hi"` would
  arrive as two arguments.
- **Rebuilding only the crannog's container:** a changed proxy image would
  only take effect after `destroy`.
- **Starting the crannog in `apply` to check that it boots:** costs a VM
  start; the next session reports a broken setup anyway.
- **Leaving a failed build in place:** the user has to clean up or
  `destroy` a crannog that never worked.
- **No `destroy` without a successful check, or a separate `--force`:** a
  broken crannog could not be destroyed, or needs another flag.
- **Remote-tracking refs as `[branch]`:** unclear what the start branch is.
- **A `CRANNOG_HOME` override or a temporary `HOME` for the smoke test:** a
  new feature, or untested with the `container` CLI.

## Consequences

- Sessions can run any command line without crannog interpreting it.
- `apply` with a running session ends it on a rebuild after a confirmation;
  the crannog then stays stopped.
- A crannog that cannot start can still be destroyed, at the risk of losing
  unfetched work.
- Adding an agent needs a change to crannog, not only to the preset.
