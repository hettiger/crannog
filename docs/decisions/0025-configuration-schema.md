# 0025. Configuration schema and setup hash

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

`config.yml` ([ADR 0023](0023-repository-configuration.md)) needs a schema
and a way to refer to paths crannog chooses: some on the host, others inside
the crannog. Users may also want to pass values from the host.

Approved setups, rebuilds and images depend on a hash of `.crannog/`, which
must not differ between the working tree and a commit with the same files.

## Decision

- **Schema:** `session.command`, `resources.cpus` and `resources.memory`,
  `proxy.image`, `env` and `mounts`. The format is documented in
  [setup](../features/setup.md).
  - `session.command` is a string, run by the agent user's login shell.
  - Resources are required and passed to the runtime as they are. crannog
    sets the proxy's resources itself.
  - Unknown keys are an error.
- **Variables provided by crannog:** `CRANNOG_*` variables name the host
  paths crannog chooses (agent homes, caches, the crannog's CA directory),
  for mount sources, and the repository path and proxy URL inside the
  crannog. They cannot be overridden. Fixed paths inside the crannog are
  written literally.
- **Interpolation like Docker Compose:** `$VAR`, `${VAR}`,
  `${VAR:-default}` and `$$`; all other variables come from the environment
  crannog is called in. Unlike Compose, an unset variable without a default
  is an error. All values are interpolated except `session.command`, which a
  shell inside the crannog expands. Values from the host are read when the
  crannog is built or rebuilt.
- **Nothing implicit:** the crannog's environment contains only what `env`
  sets; crannog adds no variables of its own.
- **Setup hash:** sha256 over a canonical form of the files: sorted paths,
  contents and the executable bit. Symlinks are an error. In the working
  tree, crannog reads the files `git ls-files -co --exclude-standard --
  .crannog` lists, so ignored files such as `.DS_Store` count neither for the
  hash nor for the image.
- **Image name:** `crannog-<hash12>`, the first twelve hex characters of the
  hash of `container/` only. Crannogs of all repositories with the same
  `container/` share the image. Destroy does not delete it.

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
- **Image name from the hash of all of `.crannog/`:** every change to
  `config.yml` or `iron-proxy.yaml` creates another name for the same image.

## Consequences

- The `default` preset spells out everything crannog relies on apart from
  the two volumes ([ADR 0018](0018-volumes-and-repository-path.md)) and the
  network.
- A changed host variable takes effect on the next rebuild. It does not
  change the setup hash and does not trigger a rebuild by itself.
- Images are built once per Mac and `container/` content. Agents start at
  the version of the build and update themselves from there.
- Unused images remain until they are cleaned up.
