# Spike: iron-proxy and the agent home

Evidence for [ADR 0007](../0007-iron-proxy-proxy-causeway.md) and
[ADR 0008](../0008-agent-home.md). Rerun it after updating the pinned
iron-proxy image or Apple Containers.

## What it checks

1. **Images:** agent image with Claude Code and Codex; iron-proxy pulled by
   digest.
2. **CA per crannog:** generated on the host via `iron-proxy generate-ca`; the
   host's keychains do not contain it.
3. **Egress via iron-proxy:** explicit proxy by IP, TLS interception with the
   crannog's CA, allowlisted domain passes, others get `403`, live reload via
   the management API, `warn: true` opens the proxy and logs unlisted hosts,
   management API and metrics not reachable from the agent.
4. **Credential injection:** `inject` scoped by host and path, `replace` swaps
   a placeholder for the real secret, `require` rejects requests without it,
   secret rotation via file and reload. Uses
   [postman-echo.com](https://postman-echo.com) to echo request headers. The
   secrets are random test values.
5. **Agent user and agent home:** non-root user with passwordless `sudo`, bind
   mount ownership and write access, two crannogs sharing the agent home,
   symlinks into it, symlinks created in a crannog as seen on the host.
6. **Manual (`--manual`):** subscription logins of Claude Code and Codex in
   one crannog, reused in the other.

The agent user is also in group `root`, to test whether that is needed for
write access. It is not.

## Running

Requires Apple Silicon, macOS 26, the `container` CLI and internet access. Run
it directly on the Mac, not inside a macOS VM.

```bash
./spike.sh            # automated checks, cleans up afterwards
./spike.sh --keep     # leaves containers running for inspection
./spike.sh --manual   # like --keep, then prints the subscription login steps
```

## Result

2026-10-31, `container` 1.6.1, iron-proxy 0.54.0: **37 passed, 0 failed.**

- `container exec` inherits the environment of `container run`.
- The bind mount appears as `agent:agent` in the crannog; files written there
  belong to the user on the host.
- A symlink created in the crannog appears on the host as a symlink to a host
  path.
- `~/.claude.json` stays a symlink after Claude Code writes it.
- Manual: Claude Code and Codex subscription logins (`/login`,
  `codex login --device-auth`) in one crannog worked in the other without
  logging in again.
- Codex keeps SQLite databases in WAL mode in `~/.codex`, and polls
  `chatgpt.com` frequently, which makes the proxy's audit log grow fast.
