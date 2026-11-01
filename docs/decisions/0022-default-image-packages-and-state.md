# 0022. Default image: system packages and agent state

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

The default image of [ADR 0013](0013-agents-default-image.md) gives the
agent `sudo`, so it can install system packages with `apt-get`. `sudo`
resets the environment, so `apt-get` runs without the proxy variables and
cannot reach anything. Debian's package sources are not in the default
allowlist, and they use plain `http://`.

The default `config.yml` points Codex and OpenCode to directories in `/state`
(`CODEX_SQLITE_HOME`, `OPENCODE_DB`). `/state` is a volume and starts empty.
Codex creates its directory; OpenCode fails to start without it.

## Decision

- **`sudo` keeps the proxy variables:** the image adds
  `Defaults env_keep += "HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy
  https_proxy no_proxy"` to `/etc/sudoers.d/proxy`, with a comment why.
- **Allowlist:** the default allowlist of ADR 0013 gets `deb.debian.org`,
  which also serves Debian's security updates (`debian-security`). The other
  entries stay.
- **apt sources** stay as Debian ships them, with `http://`. The proxy
  forwards plain HTTP and applies the allowlist to it; apt verifies package
  signatures.
- **Agent state directories:** the entrypoint creates `/state/codex` and
  `/state/opencode` after taking ownership of the volumes. The entrypoint and
  `config.yml` are both part of the setup and must match.

A [spike](0022-default-image-packages-and-state/README.md) confirmed on
Apple Containers that `sudo apt-get` works via the proxy with these
variables and fails without them, and that the allowlist applies to plain
HTTP.

## Alternatives

- **Switching apt sources to `https://`:** works too, but changes the image
  without adding security.
- **Putting OpenCode's database directly into `/state`:** needs no directory,
  but anything else OpenCode may put there later lands loose in `/state`, and
  each agent no longer has its own directory.
- **Creating only OpenCode's directory:** Codex would depend on creating its
  own.

## Consequences

- The agent can install system packages; they last until the crannog is
  rebuilt.
- Changing `CODEX_SQLITE_HOME` or `OPENCODE_DB` in `config.yml` needs the
  same change in the entrypoint.
