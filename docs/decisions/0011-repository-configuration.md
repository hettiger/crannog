# 0011. Repository configuration in `.crannog/`

- **Status:** Accepted, partially superseded by [0014](0014-one-crannog-per-start-branch.md) (crannog names: `CRANNOG_NAME`, `crannogs/<name>/`), [0015](0015-presets.md) (default on setup, source of `.crannog/`) and [0019](0019-configuration-schema.md) (variables)
- **Date:** 2026-11-01

## Context

A crannog needs an image, environment variables, mounts and a proxy
configuration. Some of these crannog needs itself (proxy, CA, agent homes,
caches), others depend on the repository. The configuration should be
transparent and flexible from the start, without compose.

The repository also lives inside the crannog
([ADR 0010](0010-git-causeway.md)). An agent can change and commit its
configuration, and the user may merge that change without noticing. A new
mount such as the user's home would break the crannog's isolation.

## Decision

- **Per repository, in the repository:** each
  [repository](../glossary.md#repository) has a `.crannog/` directory. There
  is no global configuration.

  ```
  .crannog/
    config.yml        # session, proxy image, env, mounts
    iron-proxy.yaml   # native iron-proxy configuration
    container/        # build context
      Dockerfile
      ...             # files the Dockerfile copies
  ```

- **`config.yml`** (YAML) holds the [session](../glossary.md#session)
  command, the iron-proxy image reference with digest, environment variables
  and mounts. It also lists what crannog itself needs (proxy variables, CA,
  agent homes, caches), so nothing is hidden. crannog writes a default when it
  sets up a repository.
- **Variables:** values in `config.yml` can use variables crannog provides:
  `CRANNOG_NAME`, `CRANNOG_PROXY_URL`, `CRANNOG_REPO_DIR`,
  `CRANNOG_STATE_DIR`, `CRANNOG_AGENTS_DIR`, `CRANNOG_CACHE_DIR`,
  `CRANNOG_CA_DIR`.
- **`iron-proxy.yaml`** is iron-proxy's own format, without an abstraction on
  top. crannog writes a default with the conventions it relies on: listen
  port, CA at `/etc/iron-proxy/ca.*`, management API on loopback, secrets as
  files under `/etc/iron-proxy/secrets/`. Secret values never appear in the
  file.
- **Build context:** `container/` only. Other files in `.crannog/` never reach
  the image.
- **Source on build:** if the crannog's start branch is checked out, crannog
  reads `.crannog/` from the working tree, uncommitted changes included.
  Otherwise it reads it from the start branch's latest commit. The same rule
  applies when rebuilding a crannog.
- **Approval:** crannog only uses an approved copy of `.crannog/`. If the
  content differs from every approved version, crannog shows a diff of all
  files (`git diff --no-index`) and asks before it builds. A new crannog is
  compared with the most recent approval, an existing one with its own; the
  very first build shows everything. Each content is approved once, whichever
  branch it comes from.
- **Only the approved copy is used:** the image is built from it, the
  configuration is read from it, and the proxy receives a copy of its
  `iron-proxy.yaml`. Changes to the proxy configuration of a running crannog
  go through the same approval, then crannog writes the file atomically and
  reloads the proxy without a restart.
- **State on the host:**

  ```
  ~/.crannog/repositories/<id>/       # id: sha256 of the git common dir
    path                              # repository path, for humans and cleanup
    approved/<sha256>/                # approved copies of .crannog/, by content
    approved/latest                   # hash of the most recent approval
    crannogs/<name>/
      crannog                         # hash of the copy this crannog uses
      proxy/                          # mounted into the proxy: CA, secrets, config
      ca/                             # mounted into the crannog: ca.crt only
  ```

  `<id>` is derived from the git common directory, so all git worktrees of a
  repository share it. Destroy removes `crannogs/<name>/`; removing
  `repositories/<id>/` removes everything crannog knows about a repository.
  crannog never follows symlinks in these directories.

## Alternatives

- **Compose file:** not available with Apple Containers
  ([ADR 0006](0006-apple-containers-runtime.md)).
- **Global configuration in `~/.crannog/`:** not needed; per-repository
  configuration covers it and stays versioned.
- **TOML:** the user prefers YAML.
- **Wrapping iron-proxy's configuration in `config.yml`:** loses features and
  costs work on every iron-proxy update.
- **`.crannog/` as build context:** everything in `.crannog/` would reach the
  builder.
- **Approval by hash only:** a hash shows that something changed, not what.
- **One approval per repository path:** does not work with branches that have
  different setups.
- **Mounting the project's `iron-proxy.yaml` into the proxy:** a merged change
  would take effect on the next reload without approval.

## Consequences

- The setup of a crannog is visible and versioned with the repository and can
  differ per branch.
- Changes to `.crannog/`, including the agent's, never take effect without the
  user seeing a diff.
- Users can break crannog by changing the values it relies on; nothing is
  enforced.
- Moving a repository changes its `<id>`; crannog treats it as a new
  repository.
- Removing a repository from disk leaves its state under `~/.crannog/` until
  it is cleaned up.
