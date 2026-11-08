# preset publish

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0013](../decisions/0013-host-file-access.md), [0026](../decisions/0026-presets.md), [0027](../decisions/0027-cli-commands.md)

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
  new content is written next to it and replaces it completely. The old
  setup is moved aside first and restored if installing the new one fails.
  If restoring also fails, the error names the preserved backup. A crash
  between moves can leave it at `.crannog.tmp-*/old`; directory replacement
  is not atomic.
- Needs no container runtime. The written files are not committed.
