# 0020. Proxy address in a crannog

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

The agent reaches the proxy causeway via `HTTP(S)_PROXY` pointing to the
proxy's IP ([ADR 0007](0007-iron-proxy-proxy-causeway.md)), and
`CRANNOG_PROXY_URL` is `http://<proxy IP>:8080`
([ADR 0019](0019-configuration-schema.md)). The crannog's environment is set
when its container is created.

Crannogs stop when their last session ends and start again with the next
one ([ADR 0014](0014-one-crannog-per-start-branch.md)). On Apple Containers
1.6.1, every start gives the proxy and the crannog new IPs, whatever the
order. After the first restart, the crannog cannot reach the proxy at the
address in its environment.

## Decision

- **Fixed name:** `CRANNOG_PROXY_URL` is `http://crannog-proxy:8080` and
  never changes.
- **`/etc/hosts`:** before every session, crannog makes sure the crannog's
  `/etc/hosts` maps `crannog-proxy` to the proxy's current IP and writes the
  entry as root via `container exec` if it is missing or differs. The runtime
  regenerates the file on every start, so after a start the entry is always
  written anew.
- **Default DNS:** crannogs keep the DNS configuration of Apple Containers
  (no `--no-dns`). The internal network resolves no external names, so DNS
  is no way past the proxy.

A [spike](0022-default-image-packages-and-state/README.md) confirmed this
on `container` 1.6.1: curl, Node and `sudo apt-get`
reach the proxy by name, also after stop and start; neither the default DNS
nor `--no-dns` resolves external names.

## Alternatives

- **A fixed IP for the proxy:** `container` 1.6.1 cannot assign one, and the
  allocated IPs change on every start.
- **A network alias for the proxy:** without a DNS domain configured for
  `container` on the host, an alias must be a fully qualified name, and it is
  unverified whether it resolves. Configuring a domain is a global setting
  of the user's machine.
- **Passing the proxy variables on every `container exec`:** splits the
  environment between creation and session start; a manual `container exec`
  would use a stale address.
- **Recreating the crannog on every start:** loses its writable layer.

## Consequences

- The crannog's environment stays as created; no variable depends on the
  run time.
- A crannog started with `container start` instead of crannog cannot reach
  its proxy until crannog starts a session in it.
- Every session start costs one `container exec`.
- The agent can change `/etc/hosts` with `sudo`. This gains nothing: the
  proxy is the only reachable host on the internal network.
