# 0019. Configuration schema and content hash

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

[ADR 0011](0011-repository-configuration.md) names what `config.yml` holds
(session command, proxy image, environment, mounts) and lists variables
crannog provides, but defines no schema. Some of its variables are paths on
the host, others paths inside the crannog, and `CRANNOG_NAME` has had no
meaning since crannogs lost their names
([ADR 0014](0014-one-crannog-per-start-branch.md)).

Approved copies, rebuilds and images depend on a hash of `.crannog/`, which
must not differ between the working tree and a commit with the same files.

## Decision

- **Structure:**

  ```yaml
  # crannog always attaches two volumes: /work (the repository) and /state
  # (per-crannog agent state), and the proxy as the only network.
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

  - `session.command` is a string, run by the agent user's login shell.
    `crannog checkout -- command...` replaces it.
  - `resources.cpus` and `resources.memory` are required and passed to
    `container run --cpus` and `--memory` as they are. The `default` preset
    uses 4 CPUs and 4G. crannog sets the proxy's resources itself.
  - `env` is a map, `mounts` a list of `source`, `target` and `readonly`. A
    mount's `source` may start with `~/` for the user's home; relative paths
    are an error.
  - Unknown keys are an error.
  - The agent user is always `agent` ([ADR 0008](0008-agent-home.md)).
  - `/work` and `/state` ([ADR 0012](0012-crannog-names-volumes-paths.md))
    are fixed and not part of `config.yml`: the git causeway depends on
    `/work`.

- **Variables provided by crannog:**

  | Variable | Value | Meaning |
  | -------- | ----- | ------- |
  | `CRANNOG_AGENTS_DIR` | `~/.crannog/agents` | Host path, for mounts |
  | `CRANNOG_CACHE_DIR` | `~/.crannog/cache` | Host path, for mounts |
  | `CRANNOG_CA_DIR` | `~/.crannog/repositories/<id>/crannogs/<slug>/ca` | Host path, for mounts |
  | `CRANNOG_REPO_DIR` | `/work/<host path>` | Path in the crannog |
  | `CRANNOG_PROXY_URL` | `http://<proxy IP>:8080` | URL in the crannog |

  Fixed paths inside the crannog, such as `/home/agent/.crannog/agents` or
  `/state/codex`, are written literally. This replaces the variable list of
  ADR 0011; `CRANNOG_NAME` and `CRANNOG_STATE_DIR` no longer exist.

- **Interpolation like Docker Compose:** `$VAR`, `${VAR}`,
  `${VAR:-default}` and `$$` for a literal `$`; any other `$` is an error.
  `CRANNOG_*` variables come from crannog and cannot be overridden; all
  others come from the environment crannog is called in. Unlike Compose, an
  unset variable without a default is an error. All values are interpolated
  except `session.command`, which a shell inside the crannog expands.
  Values from the host are read when the crannog is built or rebuilt.
- **Nothing implicit:** the crannog's environment contains only what `env`
  sets; crannog adds no variables of its own.
- **Host variables in approvals:** the
  [setup approval](../glossary.md#setup-approval) lists the variables a
  setup reads from the host environment.
- **Content hash:** sha256 over a canonical form of the files: sorted paths,
  contents and the executable bit. Symlinks are an error. In the working
  tree, crannog reads the files `git ls-files -co --exclude-standard --
  .crannog` lists, so ignored files such as `.DS_Store` count neither for the
  hash nor for the image.
- **Image name:** `crannog-<hash12>`, the first twelve hex characters of the
  content hash of `container/` only. Crannogs of all repositories with the
  same `container/` share the image. Destroy does not delete it.

## Alternatives

- **List form for `session.command`:** no shell syntax, and the session
  runs in a login shell anyway.
- **Optional resources with Apple Containers' default (1 GiB, 4 CPUs):**
  too small for agents and builds, and an omitted value looks unlimited.
- **Only `CRANNOG_*` variables:** safer, but the user cannot pass anything
  from the host explicitly. The approval shows which host variables are
  read.
- **Leaving unset variables empty, as Compose does:** silently puts empty
  values into a crannog.
- **`/work` and `/state` in `config.yml`:** a changed path breaks the git
  causeway; nobody needs another path.
- **Image name from the hash of all of `.crannog/`:** every change to
  `config.yml` or `iron-proxy.yaml` creates another name for the same image.

## Consequences

- The `default` preset spells out everything crannog relies on apart from
  the two volumes and the network.
- A changed host variable takes effect on the next rebuild. It does not
  change the content hash and does not trigger a rebuild by itself.
- Images are built once per Mac and `container/` content. Agents start at
  the version of the build and update themselves from there.
- Unused images remain until they are cleaned up.
