# SSH causeway

- **Status:** Implemented
- **Use cases:** [Work in a crannog with tools on the host](../use-cases/host-tools.md)
- **Decisions:** [0032](../decisions/0032-ssh-causeway.md), [0014](../decisions/0014-host-stays-out-of-guest-writable-directories.md), [0019](../decisions/0019-sessions-and-auto-stop.md), [0021](../decisions/0021-crannog-state-on-the-host.md)

## Summary

Lets SSH-based tools on the host reach a running crannog by name: `ssh`,
`scp`, `sftp`, editors with remote development (e.g. Zed) and terminal
multiplexers. It runs over `container exec`, without a network port.

## Usage

Once, add this line at the top of `~/.ssh/config`, before any `Host` or
`Match` block:

```
Include ~/.crannog/ssh/config
```

Alternatively copy single Host blocks from
`~/.crannog/repositories/<id>/crannogs/<slug>/ssh/config` into
`~/.ssh/config`, before any `Host *`.

Then start the crannog and connect to the name `crannog list` shows under
`SSH HOST`:

```sh
crannog checkout -d
crannog list
ssh crannog-1a2b3c4d-main-3f2a1b0c
```

### Zed

In Zed's settings, with the host from `crannog list` and the repository's
path in the crannog (`/work` followed by its path on the host):

```json
{
  "ssh_connections": [
    {
      "host": "crannog-1a2b3c4d-main-3f2a1b0c",
      "nickname": "my-app main",
      "upload_binary_over_ssh": true,
      "projects": [{ "paths": ["/work/Users/me/code/my-app"] }]
    }
  ]
}
```

`upload_binary_over_ssh` makes Zed download its server on the Mac and
upload it, so `zed.dev` need not be on the proxy's allowlist. Zed opens a
remote project in Restricted Mode; once trusted, it may download language
servers in the crannog. If the allowlist blocks them, set `warn: true` in
`.crannog/iron-proxy.yaml` for a while, apply it with `crannog apply` and
add what the proxy's log shows.

### Rex

Install Rex's server into the crannog from the Mac with
`rex server install ssh://<host>`. It copies the binary into the
container's file system (a rebuild removes it), then fails to set up a
service: a crannog has no systemd. Start the server over SSH instead,
passing it the crannog's environment (Rex gives its server only the
variables named with `--env`):

```sh
ssh crannog-1a2b3c4d-main-3f2a1b0c 'rex server start $(env | sed -n "s/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/--env \1/p")'
```

The server runs until the crannog stops. Add the host in Rex with
`rex hosts add <host> ssh://<host>`.

## Behavior

- **Files on the host**, readable by the user only, written by the host
  only, never mounted into a crannog:

  ```
  ~/.crannog/ssh/                       0700
    config                              Include of all crannogs' Host blocks
    id_ed25519, id_ed25519.pub          client key of all crannogs
  ~/.crannog/repositories/<id>/crannogs/<slug>/ssh/   0700
    config                              the crannog's Host block
    known_hosts                         its host key, by the host name
    ssh_host_ed25519_key                its private host key
  ```

  Every session start (`checkout`) writes them: keys once, the Host block
  and `known_hosts` every time, so the `ProxyCommand` names the path of
  the crannog binary that ran. `destroy` removes the crannog's directory
  and with it its files.
- **In the crannog**, every session start writes `/state/ssh/`
  (`ssh_host_ed25519_key`, `authorized_keys`) as the agent user, from
  content the host passes. If that fails, the session starts with a
  warning that SSH is not available.
- **A connection:** ssh runs the Host block's `ProxyCommand`,
  `crannog ssh-proxy <repository id> <branch slug>`. It checks the
  arguments and that the crannog runs; otherwise it fails with
  `the crannog of <branch> in <repository> is not running; start it with:
  crannog checkout -d <branch>`. It then replaces itself with
  `container exec` of a script that writes `/state/ssh/sshd_config` and
  runs `sshd -i` as the agent user. The config allows only the client
  key, sets the crannog's environment for sessions with `SetEnv` (except
  variables sshd sets itself and values with quotes, backslashes or
  control characters) and serves sftp internally. An image without sshd
  fails the connection with a message.
- **Not a session:** connections do not count in `crannog list` and do
  not keep the crannog running. They end when the crannog stops: after
  its last session, with `crannog stop`, a rebuild or `destroy`.
- **Host blocks** set `HostKeyAlias`, `User agent`, the `ProxyCommand`,
  `IdentityFile`, `IdentitiesOnly yes`, `IdentityAgent none`,
  `ForwardAgent no`, `ForwardX11 no`, `PermitLocalCommand no`,
  `StrictHostKeyChecking yes`, `UpdateHostKeys no` and
  `UserKnownHostsFile`. Paths are quoted, `%` is escaped; a path with a
  double quote or a control character cannot be written and SSH is then
  not available.

## Caveats

- **Your `Host *` still applies** where a Host block sets nothing:
  `IdentityFile` (your key is offered to a crannog that rejects
  crannog's key), `RemoteForward` (opens ports of your Mac to the
  crannog), `SendEnv`, `RemoteCommand`. Keep such options away from
  crannogs, e.g. with `Match host !crannog-*` instead of `Host *`.
- **Mounts:** with the `Include`, your `ssh` runs what the Host blocks in
  `~/.crannog/repositories/` say. Never mount that directory, or parts of
  it, writable into a crannog.
- Tools that can only connect to a host and port, without
  `ProxyCommand`, cannot use the SSH causeway.
