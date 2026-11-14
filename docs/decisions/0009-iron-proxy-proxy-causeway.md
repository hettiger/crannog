# 0009. iron-proxy as proxy causeway

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

[ADR 0008](0008-apple-containers-runtime.md) makes a proxy container the
agent's only way out of a crannog. That proxy must:

- allow egress per domain, not per IP, and apply changes without restarting
  the crannog,
- keep credentials such as GitHub or GitLab tokens out of the crannog and add
  them to outgoing requests instead,
- allow the user to open the network temporarily, e.g. while setting up a
  project,
- be open source and require no account.

The proxy parses traffic from an untrusted agent. Writing and reviewing such
code is not crannog's core problem and is easy to get wrong.

## Decision

crannog uses [iron-proxy](https://github.com/paradigmxyz/iron-proxy)
(Apache-2.0) as the proxy causeway, one proxy container per crannog.

- **Pinned image:** the setup references the image by digest; the default
  preset updates it deliberately.
- **Explicit proxy:** the agent reaches the proxy via `HTTP(S)_PROXY` (upper-
  and lowercase), at a fixed name on the internal network
  ([ADR 0010](0010-proxy-address.md)). The proxy's DNS feature stays
  disabled.
- **TLS interception with a CA per crannog:** crannog generates the CA on the
  host. The key goes to the proxy only, the certificate to the agent's system
  trust store and `NODE_EXTRA_CA_CERTS`. The host never trusts this CA.
- **Allowlist:** domain globs, default deny. Changes are applied by rewriting
  the config and calling the management API (`POST /v1/reload`).
- **Opening the proxy:** setting the allowlist to `warn: true` lets all
  requests pass and marks the unlisted ones in the audit log.
- **Credential injection:** credentials other than the agent's own login are
  read by the proxy from files and set on matching requests (host, path). The
  crannog only sees placeholders. `replace` with `require: true` rejects
  requests to a host that do not carry the placeholder.
- **Management API and metrics** listen on loopback inside the proxy
  container only; the agent cannot reach them. The management API key is
  random per proxy container and passed in its environment; crannog reads
  it from there when it reloads, so it is stored nowhere else.
- **Resources:** the proxy gets 256 MB of memory and the default number of
  CPUs; they are not configurable.

The [spike of iron-proxy](../spikes/iron-proxy-and-agent-home/README.md)
confirmed the proxy features on Apple Containers. A
[later spike](../spikes/default-image-and-network/README.md) measured the
proxy under parallel TLS downloads and an `npm install`: peak RSS 40 MB,
66 MiB of 256 MiB in `container stats`.

## Alternatives

- **Custom proxy in Go:** full control, but complex security-sensitive code
  that is hard to review.
- **Infisical agent-vault (standalone):** supports credential brokering, but
  is no longer maintained; its successor requires the Infisical platform.
- **Brokering subscription OAuth tokens in the proxy** (as Docker Sandboxes
  does): keeps agent tokens out of the crannog, but needs custom code for
  token endpoints, refresh and single-use refresh tokens of each agent. No
  maintained open source proxy supports it. See
  [ADR 0011](0011-agent-home.md) for how agent logins work instead.
- **Plain allowlist proxies** (tinyproxy, Squid, smokescreen, httpjail): no
  credential injection.
- **Generating the CA with `iron-proxy generate-ca`:** the proxy image would
  write into a directory the host reads
  ([ADR 0014](0014-host-stays-out-of-guest-writable-directories.md)).
- **Storing the management API key on the host:** another secret file; the
  API is on loopback in the proxy's VM anyway.
- **Configurable proxy resources:** no need so far.

## Consequences

- Secrets such as GitHub or GitLab tokens never enter a crannog. A compromised
  agent can still use them through the proxy, but only for the hosts and paths
  they are injected for.
- Every TLS connection from the crannog is intercepted. Tools must trust the
  crannog's CA; tools with certificate pinning or their own trust store need
  extra configuration.
- iron-proxy is young (0.x, frequent releases); updates need review.
- The proxy's audit log can grow fast (some agents poll frequently) and needs
  rotation.
- The CA does not depend on the proxy image; proxy images must read a PEM
  certificate and a PKCS #8 key.
