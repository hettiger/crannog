# list

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0019](../decisions/0019-sessions-and-auto-stop.md), [0021](../decisions/0021-crannog-state-on-the-host.md), [0027](../decisions/0027-cli-commands.md)

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

Remote cleanup requires the generated fetch mapping as well as the matching
name and URL. Unexpected mappings are reported and left untouched.
