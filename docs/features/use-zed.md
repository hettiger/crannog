# use zed

- **Status:** Implemented
- **Use cases:** [Work in a crannog with tools on the host](../use-cases/host-tools.md)
- **Decisions:** [0027](../decisions/0027-cli-commands.md), [0031](../decisions/0031-process-starts.md), [0032](../decisions/0032-ssh-causeway.md)

## Summary

Opens the repository in the running crannog of a branch in Zed, as a
remote project over the [SSH causeway](ssh.md).

## Usage

```sh
crannog use zed [branch]
```

`branch` defaults to the checked-out branch and completes to the start
branches of the repository's crannogs, running or not:

```sh
crannog checkout -d
crannog use zed
crannog use zed feature/x
```

It needs the `Include` of the SSH causeway in `~/.ssh/config` and Zed's
command line tool `zed` in `PATH` (in Zed: Zed > Install CLI). Close the
project in Zed; there is no command for it.

### Zed's settings

Zed takes the settings of a remote project from the entry of its host in
`ssh_connections`, if there is one:

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

`host` is the name `crannog list` shows under `SSH HOST`, the path is
`/work` followed by the repository's path on the host. With `projects`
the crannog is in Zed's list of remote projects too.

`upload_binary_over_ssh` makes Zed download its server on the Mac and
upload it, so `zed.dev` need not be on the proxy's allowlist. Zed opens a
remote project in Restricted Mode; once trusted, it may download language
servers in the crannog. If the allowlist blocks them, set `warn: true` in
`.crannog/iron-proxy.yaml` for a while, apply it with `crannog apply` and
add what the proxy's log shows.

## Behavior

1. The same checks as [`crannog ssh`](ssh.md): the crannog must exist and
   run (it never starts one), its Host block must exist, and `ssh -G`
   must show crannog's `ProxyCommand` for its host. The messages are the
   same.
2. Runs `zed ssh://<host>/<path>`, the path of the repository in the
   crannog, escaped for the URL (Zed decodes it). `zed` hands the project
   to Zed, starting it if needed, and returns; Zed connects with the
   system `ssh` and the user's SSH configuration.
3. Fails with `zed is not in PATH; install Zed's command line tool in Zed
   with Zed > Install CLI` without `zed`, and with zed's output if it
   fails.

`crannog use` without a tool prints its help; an unknown tool fails with
`unknown tool "<tool>"; crannog use supports: zed`.
