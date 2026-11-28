# Work in a crannog with tools on the host

- **Actor:** User
- **Features:** [SSH causeway](../features/ssh.md), [open](../features/open.md), [checkout](../features/checkout.md), [list](../features/list.md), [stop](../features/stop.md)

## Goal

The user works on the code in a crannog with the tools of their Mac: an
editor with remote development (Zed, later JetBrains Gateway or VS Code),
a terminal multiplexer (herdr, Rex) or plain `ssh`, while the agent keeps
working in the same crannog.

## Scenario

1. The user enables SSH to crannogs once in their SSH configuration.
2. The user starts the crannog of a branch in the background.
3. The user connects the tool to the crannog by name and edits, runs
   commands and uses terminals in the crannog.
4. The user stops the crannog when done; the tool's connection ends.

## Acceptance criteria

- crannog works without the SSH configuration; enabling it is the user's
  decision.
- The user's keys, ssh-agent and SSH settings do not reach the crannog
  through crannog's Host blocks.
- No port or network path from the Mac into the crannog is needed.
- What the tool runs in the crannog has the crannog's environment and
  reaches the network only through the proxy.
