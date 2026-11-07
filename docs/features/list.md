# list

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0016](../decisions/0016-cli-commands.md), [0025](../decisions/0025-session-locks-and-lifecycle.md), [0032](../decisions/0032-anchored-host-file-access.md)

## Summary

Lists the repository's crannogs: start branch, status and running sessions.

## Usage

```sh
crannog list
```

## Behavior

Repairs the repository's crannogs first, then prints a table to stdout,
sorted by start branch. Status is the runtime's state (`running`,
`stopped`, …), `incomplete` for the remains of an interrupted build, or
`containers missing`. Sessions are counted on the host from
`sessions/<pid>` files that a running session holds locked. Only
directories named like a branch slug count as crannogs. Without crannogs it
says so on stderr.
