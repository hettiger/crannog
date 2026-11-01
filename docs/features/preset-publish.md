# preset publish

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0015](../decisions/0015-presets.md), [0016](../decisions/0016-cli-commands.md), [0030](../decisions/0030-poc-implementation-conventions.md)

## Summary

Writes a preset to `.crannog/` in the current worktree.

## Usage

```sh
crannog preset publish [name] [--yes]
```

## Behavior

- Without a name, shows a picker of all presets; without a terminal that is
  an error.
- If `.crannog/` exists, asks before replacing it (`--yes` answers it). The
  new content is written next to it and replaces it completely.
- Needs no container runtime. The written files are not committed.
