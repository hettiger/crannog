# 0030. Implementation conventions of the PoC

- **Status:** Proposed
- **Date:** 2026-11-01

## Context

The PoC needs details that no earlier ADR fixes: formats of the setup
hash and the repository id, how crannog talks to the proxy, how it finds a
crannog's state, how it runs sessions and what it does with incomplete
crannogs. They are implemented as described here and are open for review
with the PoC.

## Decision

- **Repository id:** sha256 in hex of the git common dir as
  `git rev-parse --path-format=absolute --git-common-dir` prints it.
- **Setup hash:** sha256 in hex over the files sorted by path; per file
  `path NUL x|- NUL length NUL content`, where `x` marks the executable
  bit. Files of the bundled presets are never executable. A setup without
  `config.yml`, `iron-proxy.yaml` or `container/Dockerfile` is an error.
- **`config.yml`:** scalars are read as written (`cpus: 4` is `"4"`).
  Interpolation supports exactly `$VAR`, `${VAR}`, `${VAR:-default}` and
  `$$`; a default cannot contain `$`; an unknown `CRANNOG_*` variable is an
  error. Mount targets must be absolute and must not be `/work`, `/state`
  or below them. `env` and `config.yml` errors are reported before any
  approval.
- **Proxy conventions** (in addition to [ADR 0011](0011-repository-configuration.md)):
  the proxy's copy is `proxy/iron-proxy.yaml`, started with
  `-config /etc/iron-proxy/iron-proxy.yaml`; crannog waits for
  `http://127.0.0.1:9090/healthz` and reloads via
  `http://127.0.0.1:9092/v1/reload`. The management API key is random per
  proxy container and passed in `IRON_MANAGEMENT_API_KEY`; crannog reads it
  from the proxy's environment when it reloads, so it is stored nowhere
  else. If the proxy rejects a new configuration, crannog restores the old
  file. crannog creates an empty `proxy/secrets/`.
- **CA:** generated with `iron-proxy generate-ca` in a throwaway container
  of the proxy image, named `crannog <container name>`; `ca.key` gets mode
  0600 on the host.
- **Runtime:** the internal network is created with `--internal` and no
  fixed subnet. Containers are created with `container create` and started
  separately; mounts use `--volume source:target[:ro]`. Status and
  addresses come from `container list --all --format json`. Waiting for the
  proxy and the readiness marker polls every 250 ms for at most 60 seconds
  and stops early if the container stopped.
- **State of a crannog:** it exists once `crannogs/<slug>/crannog` holds a
  hash. A host directory without it is the remains of an interrupted build;
  `checkout` removes them and builds anew. A crannog whose containers are
  missing is an error with recovery guidance
  ([ADR 0034](0034-bounded-readiness-and-recovery-guidance.md)).
- **Sessions:** a session runs
  `sh -c 'exec "$login_shell" -l -c "$1"'` as `agent` in
  `/work/<host path>`, with the login shell from `getent passwd` and
  `/bin/sh` as fallback. crannog ignores Ctrl-C (the terminal passes it to
  the session) and passes SIGTERM and SIGHUP on. A session killed by a
  signal gives exit code 128 + signal. crannog's own messages go to stderr.
- **Entrypoint of the default image:** it idles with a TERM trap, so
  `container stop` does not wait for its timeout.
- **`destroy`** fetches into `refs/crannog/destroy/<slug>/*` without
  writing `FETCH_HEAD` and deletes these refs afterwards. A branch counts as
  lost if it has commits not reachable from any ref except
  `refs/crannog/*` and `refs/remotes/crannog-<slug>/*`.
- **Self-repair** runs in `checkout`, `apply`, `destroy` and `list`, not in
  the preset commands, which work without the runtime.
- **Approval diff:** `git diff --no-index --no-prefix` between directories
  named `approved` and `.crannog`, with git's pager in a terminal.
- **`preset publish`** writes the new `.crannog/` next to the old one and
  replaces it only then.

## Alternatives

- **Storing the management API key on the host:** another secret file; the
  API is on loopback in the proxy's VM anyway.
- **Running sessions with a fixed shell (`zsh -l -c`):** ties crannog to the
  default image.
- **Writing `FETCH_HEAD` in `destroy`:** overwrites the user's.

## Consequences

- Images other than the default must provide `sh`, `getent` or a passwd
  entry for `agent`, and an entrypoint that writes the readiness marker.
- Proxy images other than iron-proxy's must provide `sh` and `wget`.
