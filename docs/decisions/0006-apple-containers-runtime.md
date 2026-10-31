# 0006. Apple Containers as sandbox runtime

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

A crannog must contain an agent that runs without supervision. Linux kernel
vulnerabilities are frequent, so a shared kernel is a weak boundary: on macOS,
Docker engines run all containers in one Linux VM, and an escape reaches every
other container in it and every host path the VM mounts.

Requirements: macOS only (Apple Silicon, macOS 26 acceptable), one isolated
environment per crannog, network egress enforced from outside the agent's
environment, open source, no account required, interactive sessions, fast
startup.

## Decision

crannog uses [Apple Containers](https://github.com/apple/container)
(`container` CLI, Apache-2.0) as its only runtime.

- Each crannog runs in its own lightweight VM with its own kernel.
- crannog requires Apple Silicon and macOS 26 or later.
- No runtime abstraction layer. crannog talks to `container` directly.
- No compose. Additional services (e.g. a database) run inside the agent's
  container.
- Egress is enforced by network topology: the agent's container is attached
  only to an internal (host-only) network per crannog. A proxy container is
  attached to that network and to the default network and is the agent's only
  way out.

A [spike](0006-apple-containers-runtime/README.md) confirmed: direct egress is blocked (by name and by IP, also for
root, which lacks `NET_ADMIN`), traffic via the proxy follows a domain
allowlist that can change without restart, the agent cannot reach the LAN,
and a warm start takes about a second.

## Alternatives

- **Docker / Docker Compose** (OrbStack, Docker Desktop, Colima): mature,
  compose and sidecars are easy, but all containers share one kernel.
- **OrbStack isolated machines:** no host file system by default, but one
  shared kernel; OrbStack states they are not a full security boundary.
  Proprietary.
- **Docker Sandboxes (`sbx`):** microVM per sandbox with host-side proxy and
  credential injection, but proprietary and requires a Docker account login.
- **Lima or full VMs per crannog:** strong isolation, but heavy, slow to boot,
  and every integration would have to be built from scratch.
- **Community compose plugins for Apple Containers:** not first-party, not
  needed if services run in the agent's container.

## Consequences

- Strong isolation per crannog: an escape from the container ends in that
  crannog's VM, which holds only what crannog put into it.
- macOS 26 on Apple Silicon only. No Linux or Intel support.
- crannog wires networking and the proxy itself; there is no compose file.
- Services on the Mac that listen on `0.0.0.0` are reachable from a crannog via
  the internal network's gateway and the Mac's LAN IP. Services bound to
  `127.0.0.1` are not.
- The first `container system start` asks to install a default kernel.
- VM memory is not returned to macOS until a crannog restarts.
- crannog depends on a young project (1.x, fast-moving CLI and output formats).
