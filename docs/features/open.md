# open

- **Status:** Implemented
- **Use cases:** [Work in a crannog with tools on the host](../use-cases/host-tools.md)
- **Decisions:** [0027](../decisions/0027-cli-commands.md), [0030](../decisions/0030-seatbelt.md), [0031](../decisions/0031-process-starts.md), [0032](../decisions/0032-ssh-causeway.md)

## Summary

Opens the running crannog of a branch in the browser, at its address, or
with `--zed` the repository in it in Zed, as a remote project over the
[SSH causeway](ssh.md).

## Usage

```sh
crannog open [branch] [--insecure] [--port <port>]
crannog open [branch] --zed
```

`branch` defaults to the checked-out branch and completes to the start
branches of the repository's crannogs, running or not:

```sh
crannog checkout -d
crannog open                          # https://<address>
crannog open --insecure --port 8080   # http://<address>:8080
crannog open feature/x --zed
```

Which server answers in the crannog is up to its setup. `--zed` needs the
`Include` of the SSH causeway in `~/.ssh/config`. Close the browser tab or
the project in Zed; there is no command for it.

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

`host` is what `crannog ssh --host` prints, the path what follows the
colon in `crannog ssh --path`. With `projects` the crannog is in Zed's
list of remote projects too.

`upload_binary_over_ssh` makes Zed download its server on the Mac and
upload it, so `zed.dev` need not be on the proxy's allowlist. Zed opens a
remote project in Restricted Mode; once trusted, it may download language
servers in the crannog. If the allowlist blocks them, set `warn: true` in
`.crannog/iron-proxy.yaml` for a while, apply it with `crannog apply` and
add what the proxy's log shows.

## Behavior

- **Browser:**
  1. Fails if the branch has no crannog (`there is no crannog for
     <branch>`) or if it is not running, with the message of
     [`crannog ssh`](ssh.md). It never starts a crannog.
  2. Takes the crannog's address on its internal network. The Mac
     reaches it; the address can change from start to start.
  3. Checks that something accepts a TCP connection on the address and
     port (443, 80 with `--insecure`, or `--port`) within 2 seconds,
     without sending anything. Otherwise it fails with `nothing answers on
     <url>` and opens nothing.
  4. Runs `/usr/bin/open <url>`: `https://<address>`, `http://` with
     `--insecure`, `:<port>` with `--port`.
- **Zed:**
  1. The same checks as `crannog ssh`, with the same messages: the
     crannog exists and runs, its Host block exists, and `ssh -G` shows
     crannog's `ProxyCommand` for its host.
  2. Runs `/usr/bin/open zed://ssh/<host><path>`, with the path of the
     repository in the crannog escaped for the URL (Zed decodes it).
     macOS hands the URL to Zed, starting it if needed; Zed connects with
     the system `ssh` and your SSH configuration. Zed's own command line
     tool `zed` cannot be used: it starts Zed in a way that fails in any
     sandbox, Seatbelt too.
- If `open` fails, e.g. without Zed, the error names the URL and `open`'s
  message.
- `--zed` with `--insecure` or `--port`, and a port outside 1–65535, are
  errors.
