# stop

- **Status:** Implemented
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0019](../decisions/0019-sessions-and-auto-stop.md), [0027](../decisions/0027-cli-commands.md)

## Summary

Ends the sessions in the crannog of a start branch and stops the crannog.
Nothing is deleted.

## Usage

```sh
crannog stop [branch]
```

`branch` defaults to the checked-out branch; it may name a branch that no
longer exists. Without a crannog for the branch, stop is an error.

## Behavior

1. Takes the crannog's lock, waiting for other crannog commands, then
   repairs the repository's other crannogs like [checkout](checkout.md).
   If the crannog was destroyed meanwhile, stop is an error and leaves no
   host directory behind.
2. If the crannog is not running, says so and ends without an error.
3. Ends the running sessions like [destroy](destroy.md),
   [detached sessions](checkout.md#detached-sessions) and sessions on
   terminals alike, and waits until none runs, without a timeout. It does
   not ask for confirmation. Ctrl-C ends stop with the crannog still
   running; the processes in the crannog were asked to end all the same.
4. Stops crannog and proxy and removes the remote, as after the last
   session. Volumes, containers and the host directory stay; the next
   [checkout](checkout.md) starts the crannog again.

A `crannog checkout` whose session stop ended exits with the session's exit
code and finds nothing left to stop.
