# Spike: default image and network

Evidence for [ADR 0009](../../decisions/0009-iron-proxy-proxy-causeway.md) (proxy
memory), [ADR 0010](../../decisions/0010-proxy-address.md),
[ADR 0017](../../decisions/0017-one-crannog-per-start-branch.md) (name limits) and
[ADR 0028](../../decisions/0028-agents-in-the-default-image.md) (system packages). Rerun
it after updating Apple Containers.

`container/` is the draft of crannog's default image from the
[agents spike](../agents-default-image/README.md) with `sudo` keeping the
proxy variables and `deb.debian.org` allowlisted. `spike.sh` checks repository paths, name limits, plain HTTP and
`sudo apt-get` via the proxy, exit codes, agent state in `/state` and the
proxy's memory. `network.sh` checks how a crannog reaches its proxy by name,
DNS in a crannog and what an app preview could build on. It reuses the image
built by `spike.sh`.

## What it checks

`spike.sh`:

1. **Repository path:** git's common dir after `cd` with another case, from
   a subdirectory, from a worktree and through a symlinked path.
2. **Name limits** of containers, networks and volumes.
3. **Resources:** proxy with 256M, crannog with 4 CPUs and 4G.
4. **Plain HTTP and apt:** plain HTTP via the proxy's tunnel listener,
   allowlist applied to it, `sudo` keeping the proxy variables,
   `sudo apt-get` via the proxy, apt with `https://` sources.
5. **Exit codes** of `container exec`.
6. **Agent state:** Codex and OpenCode in the dirs in `/state`.
7. **Proxy memory** under parallel TLS downloads and an `npm install`.
8. **Proxy IP** after stop and start.

`network.sh`:

1. **`/etc/hosts`:** how it is provided, written by root.
2. **Proxy by name** via `http://crannog-proxy:8080` with curl, Node and
   `sudo apt-get`, with and without `--no-dns`.
3. **DNS:** whether a crannog resolves external names.
4. **Stop and start:** the `/etc/hosts` entry, rewriting it.
5. **Network aliases.**
6. **App preview basics:** the Mac reaching a server in a crannog; DNS
   domain and names, read only.

## Running

Requires Apple Silicon, macOS 26, the `container` CLI and internet access.

```bash
./spike.sh            # first: builds the image, cleans up afterwards
./network.sh          # reuses the image
```

Both accept `--keep` to leave everything running.

## Result

2026-11-01, `container` 1.6.1, macOS 26.5.1, Apple Git 2.50.1.

`spike.sh`: **31 passed, 2 failed.** The two failures were the proxy IP
checks (section 8), now reported as findings.

- git returns the on-disk case and the physical path (`/private/var/...`)
  after `cd` with another case, from a subdirectory and from a worktree.
- Names: containers and networks at most 63 characters, volumes 255
  (`^[A-Za-z0-9][A-Za-z0-9_.-]*$`).
- Plain HTTP via the tunnel listener works and the allowlist applies.
  `sudo` keeps the proxy variables; without them `apt-get` fails. apt with
  `https://` sources works as well.
- `container exec` passes the exit code, also with `-t`; SIGTERM gives 143,
  an unknown command 1.
- Codex created `/state/codex` itself. OpenCode, without `/state/opencode`,
  failed with "Background service failed to start" and created no database.
- Proxy: peak RSS 40 MB after 40 downloads of 4 MB (20 in parallel) and an
  `npm install`; `container stats` showed 66 MiB of 256 MiB.
- With `--cpus 4`, `nproc` in the crannog showed 5; with `--memory 256M`,
  the proxy's VM showed 349 MiB.
- Every start gives the proxy and the crannog new IPs, also when the proxy
  starts first; the crannog cannot reach the proxy at its old IP.

`network.sh`: **29 passed, 0 failed.**

- The runtime generates `/etc/hosts` on every start (`localhost` and the
  container's own name). root can add entries; they are gone after stop and
  start; writing them again works.
- curl, Node and `sudo apt-get` reach the proxy via `crannog-proxy`, with
  and without `--no-dns`.
- Neither the default DNS (the network's gateway) nor `--no-dns`
  (`1.1.1.1`) resolves external names or container names.
- An alias without a DNS domain is rejected: it "must be a fully qualified
  name with at least 2 labels, such as "api.example.test". Configure a
  default DNS domain to have the domain added to a name that has none".
- The Mac reaches a server in a crannog by its IP on the internal network.

After the run, the entrypoint got the `mkdir` for `/state/codex` and
`/state/opencode`, and `spike.sh` was adjusted to it and to
report the proxy IP as findings. That version was not rerun.
