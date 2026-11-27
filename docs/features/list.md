# list

- **Status:** Implemented
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0019](../decisions/0019-sessions-and-auto-stop.md), [0021](../decisions/0021-crannog-state-on-the-host.md), [0027](../decisions/0027-cli-commands.md), [0032](../decisions/0032-ssh-causeway.md)

## Summary

Lists the repository's crannogs: start branch, status, running sessions
and the host name of the [SSH causeway](ssh.md).

## Usage

```sh
crannog list
```

## Behavior

Repairs the repository's crannogs first, then prints a table to stdout,
sorted by start branch. Status is the runtime's state (`running`,
`stopped`, …), `incomplete` for the remains of an interrupted build, or
`containers missing`, which the next [checkout](checkout.md) or
[apply](apply.md) recreates. Sessions are counted on the host from
`sessions/<uuid>.yml` files that a running session holds locked. Only
directories named like a branch slug count as crannogs. Without crannogs it
says so on stderr. `SSH HOST` is the crannog's runtime name, the `Host`
of its Host block.

Remote cleanup requires the generated fetch mapping as well as the matching
name and URL. Unexpected mappings are reported and left untouched.
