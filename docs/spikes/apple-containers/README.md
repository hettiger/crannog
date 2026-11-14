# Spike: Apple Containers as sandbox runtime

Evidence for [ADR 0008](../../decisions/0008-apple-containers-runtime.md). Rerun it after
Apple Containers updates to check that the ADR's assumptions still hold.

## What it checks

1. **Build:** images built from Dockerfiles outside any project directory;
   build and startup time.
2. **Egress:** the agent container is attached only to an internal network; a
   proxy container on the internal and default network is its only way out.
   Direct egress (by name and IP, also as root) is blocked, allowlisted domains
   pass the proxy, others get `403`, allowlist changes apply without restart.
3. **Host exposure:** which host and LAN services the agent can reach
   (reported as findings, not pass/fail).
4. **Git over SSH:** push into the container's repo
   (`receive.denyCurrentBranch updateInstead`), commit inside, fetch back.
   SSH uses only the generated key (`-F /dev/null`, `IdentitiesOnly=yes`,
   `IdentityAgent=none`, `ForwardAgent=no`).

tinyproxy is a test fixture only. It is not the chosen proxy.

## Running

Requires Apple Silicon, macOS 26 and the `container` CLI
(`container system start` must have run once). Run it directly on the Mac,
not inside a macOS VM: Virtualization.framework does not support nested
virtualization for macOS guests.

```bash
./spike.sh          # cleans up containers and network afterwards
./spike.sh --keep   # leaves them running for inspection
```

macOS may ask whether `nc` may accept incoming connections (used for the host
exposure check). Allow it, or the findings in step 3 are not meaningful.

## Result

2026-10-31, `container` 1.6.1: **18 passed, 0 failed.**

- Warm build 2 s (cold 48 s), agent start until SSH is reachable < 1 s.
- Root in the agent container has no `NET_ADMIN` and cannot change routes.
- Host services bound to `0.0.0.0` are reachable via the internal gateway and
  the Mac's LAN IP; services bound to `127.0.0.1` are not. The LAN router is
  not reachable from the agent (but is from the host).
