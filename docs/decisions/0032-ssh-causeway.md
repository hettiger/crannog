# 0032. SSH causeway via a ProxyCommand over `container exec`

- **Status:** Accepted
- **Date:** 2026-11-27

## Context

Users want to work in a crannog with tools on the host that connect over
SSH: editors with remote development (Zed first, later JetBrains Gateway
and VS Code Remote-SSH) and terminal multiplexers (herdr, Rex). Git and
sessions do not need SSH ([ADR 0015](0015-git-causeway.md)); these tools
do. Most of them run the system `ssh` with the user's SSH configuration,
some have a client of their own.

A crannog is attached only to an internal network
([ADR 0008](0008-apple-containers-runtime.md)), its address can change
from start to start, and crannog must not change `~/.ssh` nor offer the
user's keys or ssh-agent to a crannog ([ADR 0015](0015-git-causeway.md)).

## Decision

- **Transport:** a crannog's Host block has a `ProxyCommand` that runs
  `crannog ssh-proxy <repository id> <branch slug>` (a hidden command). It
  checks that the crannog runs and replaces itself with
  `container exec -i <crannog> … sshd -i` as the agent user: sshd serves
  that one connection on the pipe. No port, no network path from the Mac
  into the crannog, no address, no sshd running in the background.
- **Not a session, never starts a crannog:** like the git causeway, SSH
  works only while the crannog runs, e.g. after `crannog checkout -d`.
  Otherwise `ssh-proxy` fails and names that command. A connection does
  not keep the crannog running: it ends when the last session ends, with
  `crannog stop`, a rebuild or `destroy`.
- **Opt-in in the user's SSH configuration:** crannog never writes to
  `~/.ssh`. Whoever wants the SSH causeway adds
  `Include ~/.crannog/ssh/config` at the top of `~/.ssh/config`, or
  copies single Host blocks. crannog works without it. Then every tool
  that runs the system `ssh` reaches crannogs by name, and tools with a
  client of their own if it supports `ProxyCommand`.
- **Host name:** the crannog's runtime name, `crannog-<id8>-<slug>`
  ([ADR 0017](0017-one-crannog-per-start-branch.md)). `crannog list`
  shows it.
- **Keys, all generated on the host:** one client key for all crannogs in
  `~/.crannog/ssh/`, and one host key per crannog in its host directory,
  generated once each. crannog writes the crannog's `known_hosts` from the
  host key it generated: nothing from the crannog reaches the user's SSH
  files. On every session start crannog hands the private host key and
  the client's public key to the crannog, which writes them to
  `/state/ssh` itself ([ADR 0014](0014-host-stays-out-of-guest-writable-directories.md)).
- **Host blocks set everything that matters:** in an SSH config the first
  value wins, so each block sets `IdentitiesOnly yes`, its own
  `IdentityFile`, `IdentityAgent none`, `ForwardAgent no`,
  `ForwardX11 no`, `PermitLocalCommand no`, `StrictHostKeyChecking yes`,
  `UpdateHostKeys no`, its own `UserKnownHostsFile` and `HostKeyAlias`.
  Forwardings are not cleared: `ClearAllForwardings` would also drop those
  a tool asks for on the command line, and they are opened from the Mac.
- **Files on the host** are readable by the user only and are written
  only by the host; none is ever mounted into a crannog. A crannog's
  files live in its host directory, so `destroy` removes them, and
  `~/.crannog/ssh/config` includes them by a pattern.
- **The crannog's environment:** sshd starts sessions with a fresh
  environment. The script that starts sshd writes the crannog's
  variables into the sshd config (`SetEnv`), so SSH sessions have the
  proxy variables and the image's `PATH`.
- **Optional:** if preparing the SSH causeway fails, the session starts
  with a warning. An image without sshd fails only the SSH connection.
  The default image ships `openssh-server`
  ([ADR 0028](0028-agents-in-the-default-image.md)).

## Alternatives

- **sshd listening on a port, published or reached on the crannog's
  address:** needs a route from the Mac into the crannog, stable ports
  per crannog and collision handling; copied configs break when a port
  or address changes.
- **The SSH connection as a session that starts the crannog:** a
  `ProxyCommand` cannot ask for an approval or confirmation, its stdin
  and stdout are the connection; editors reconnect by themselves and
  would start a crannog the user just stopped.
- **The connection as a session that does not start the crannog:**
  keeps an editor connected when the user ends the last other session,
  but needs an exception from the confirmation before a second session,
  and a lingering background `ssh` would keep the crannog running. It
  can be added later without changing the rest.
- **Host keys generated in the crannog:** the host would have to read the
  public key from the crannog and check it before writing `known_hosts`.
  The private host key is in the crannog either way; it only proves the
  crannog's identity to the Mac.
- **A new host key per connection:** concurrent connections would
  overwrite each other's `known_hosts` entry.
- **Turning host key checking off** (the pipe already leads to the right
  crannog): tools with an SSH client of their own may ignore it and ask
  to trust the host.
- **One `hosts/` directory for all Host blocks:** `destroy` would have to
  remove entries, and concurrent checkouts would share files.
- **Writing to `~/.ssh/config` or `known_hosts`:** a broken entry affects
  every SSH connection of the user ([ADR 0015](0015-git-causeway.md)).
- **`ssh-keygen` on the host** for keys: one more program crannog starts
  ([ADR 0031](0031-process-starts.md)), and parsing its output by hand.

## Consequences

- Tools reach a crannog by its runtime name, without ports or addresses,
  as long as they run the system `ssh` or support `ProxyCommand`. A tool
  that can only connect to a host and port would need a local listener
  that crannog does not provide.
- JetBrains Gateway reads only `~/.ssh/config`, Rex probably as well: they
  need the `Include`.
- Options in the user's `Host *` that the Host block does not set still
  apply: an `IdentityFile` there is offered to a crannog that rejects the
  crannog's key (its public part, and a signature for that session if the
  crannog accepts it), `RemoteForward` opens Mac ports to the crannog,
  `SendEnv` sends variables. The docs explain how to exclude crannogs.
- The `ProxyCommand` holds the absolute path of the crannog binary that
  wrote it, since tools started from the Dock have a short `PATH`. A
  `checkout` rewrites it; copied blocks must be copied again after the
  binary moved.
- A setup that mounts parts of `~/.crannog/repositories/` writable into a
  crannog gives it a way to run commands on the Mac through the user's
  `ssh`.
- Editors that download their server or language servers into the
  crannog go through the proxy: Zed can upload its server from the Mac
  (`upload_binary_over_ssh`), language servers may need the allowlist
  opened for a while (`warn: true`).
- What runs over SSH in a crannog runs as the agent user, like a session.
