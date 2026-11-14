# Let a coding agent work autonomously

- **Actor:** User
- **Features:** [checkout](../features/checkout.md), [apply](../features/apply.md), [destroy](../features/destroy.md), [list](../features/list.md), [preset publish](../features/preset-publish.md), [git-remote-crannog](../features/git-remote-crannog.md), [default preset](../features/default-preset.md), [setup](../features/setup.md)

## Goal

The user hands a coding task to an agent and lets it work without supervision,
without risking their host system, credentials, or other data.

## Scenario

1. The user builds a crannog for a repository.
2. The agent works inside the crannog without asking for permissions.
3. The agent can reach the outside world only through causeways the user
   allowed: the git causeway, the proxy causeway and optionally mount
   causeways.
4. The user receives the agent's results via the git causeway and decides what
   to keep.

## Acceptance criteria

- The agent cannot access anything on the host that is not connected via a
  causeway.
- Every causeway is configured and controlled by the user.
