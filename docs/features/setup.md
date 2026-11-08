# setup

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0009](../decisions/0009-iron-proxy-proxy-causeway.md), [0018](../decisions/0018-volumes-and-repository-path.md), [0020](../decisions/0020-readiness-marker.md), [0023](../decisions/0023-repository-configuration.md), [0024](../decisions/0024-setup-approval.md), [0025](../decisions/0025-configuration-schema.md)

## Summary

The format of a repository's `.crannog/`: `config.yml`, `iron-proxy.yaml`
and `container/`, and what crannog expects of them.

## Usage

```
.crannog/
  config.yml
  iron-proxy.yaml
  container/
    Dockerfile
```

All three are required. `crannog preset publish` writes a preset as a
starting point.

Symlinks in `.crannog/` are an error. Text files, which `git diff` shows in
the approval diff, must not contain control characters other than tab and
line breaks: escape sequences could hide lines in the terminal. A file with
a NUL byte in its first 8000 bytes is binary, as for `git diff`, and may
contain anything.

## Behavior

### `config.yml`

```yaml
session:
  command: claude --dangerously-skip-permissions
resources:
  cpus: 4
  memory: 4G
proxy:
  image: docker.io/ironsh/iron-proxy:0.54.0@sha256:...
env:
  HTTPS_PROXY: ${CRANNOG_PROXY_URL}
mounts:
  - source: ${CRANNOG_CA_DIR}
    target: /etc/crannog
    readonly: true
```

- Scalars are read as written (`cpus: 4` is `"4"`) and passed to
  `container` as `--cpus` and `--memory`.
- A mount's `source` is absolute or starts with `~/`, and must exist. Its
  `target` is absolute and not `/work`, `/state` or below them.
- `crannog checkout -- command...` replaces `session.command`.

### Variables

| Variable | Value | Meaning |
| -------- | ----- | ------- |
| `CRANNOG_AGENTS_DIR` | `~/.crannog/agents` | Host path, for mounts |
| `CRANNOG_CACHE_DIR` | `~/.crannog/cache` | Host path, for mounts |
| `CRANNOG_CA_DIR` | `~/.crannog/repositories/<id>/crannogs/<slug>/ca` | Host path, for mounts |
| `CRANNOG_REPO_DIR` | `/work/<host path>` | Path in the crannog |
| `CRANNOG_PROXY_URL` | `http://crannog-proxy:8080` | URL in the crannog |

Interpolation supports exactly `$VAR`, `${VAR}`, `${VAR:-default}` and `$$`;
any other `$` is an error, and a default cannot contain `$`. An unknown
`CRANNOG_*` variable and an unset variable without a default are errors.

### `iron-proxy.yaml`

The proxy runs with `-config /etc/iron-proxy/iron-proxy.yaml`; crannog
mounts the crannog's `proxy/` directory read-only at `/etc/iron-proxy`. It
holds crannog's copy of `iron-proxy.yaml`, the CA (`ca.crt`, `ca.key`) and an
empty `secrets/`. crannog relies on:

- the proxy listening on `:8080` (`CRANNOG_PROXY_URL`),
- the CA at `/etc/iron-proxy/ca.crt` and `ca.key`,
- the health endpoint `http://127.0.0.1:9090/healthz`, which crannog waits
  for,
- the management API at `http://127.0.0.1:9092`, reloaded with the key in
  `IRON_MANAGEMENT_API_KEY`.

### Setup hash

sha256 in hex over the files sorted by path; per file
`path NUL x|- NUL length NUL content`, where `x` marks the executable bit.
Files of the bundled presets are never executable.

### Custom images

- The crannog's image provides `sh`, and `getent` or a passwd entry for
  `agent`; a session runs in `agent`'s login shell, else `/bin/sh`.
- Its entrypoint writes the [readiness marker](../glossary.md#readiness-marker)
  once the crannog is prepared, then idles. Without a TERM trap,
  `container stop` waits for its timeout.
- The proxy image provides `sh` and `wget` for the health check and the
  reload, reads a PEM certificate and a PKCS #8 key, and does not write its
  configuration directory.
