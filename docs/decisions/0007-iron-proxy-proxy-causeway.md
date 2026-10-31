# 0007. iron-proxy as proxy causeway

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

[ADR 0006](0006-apple-containers-runtime.md) makes a proxy container the
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

- **Pinned image:** crannog references the image by digest and updates it
  deliberately.
- **Explicit proxy:** the agent reaches the proxy via `HTTP(S)_PROXY` (upper-
  and lowercase) pointing to the proxy's IP on the internal network. The
  proxy's DNS feature stays disabled.
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
  container only; the agent cannot reach them.

A [spike](0007-iron-proxy-proxy-causeway/README.md) confirmed all of the above
on Apple Containers.

## Alternatives

- **Custom proxy in Go:** full control, but complex security-sensitive code
  that is hard to review.
- **Infisical agent-vault (standalone):** supports credential brokering, but
  is no longer maintained; its successor requires the Infisical platform.
- **Brokering subscription OAuth tokens in the proxy** (as Docker Sandboxes
  does): keeps agent tokens out of the crannog, but needs custom code for
  token endpoints, refresh and single-use refresh tokens of each agent. No
  maintained open source proxy supports it. See
  [ADR 0008](0008-agent-home.md) for how agent logins work instead.
- **Plain allowlist proxies** (tinyproxy, Squid, smokescreen, httpjail): no
  credential injection.

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
