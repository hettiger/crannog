# 0023. Repository configuration in `.crannog/`

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

A crannog needs an image, environment variables, mounts and a proxy
configuration. Some of these crannog needs itself (proxy, CA, agent homes,
caches), others depend on the repository. The configuration should be
transparent and flexible from the start, without compose.

With several git worktrees, the working tree of the start branch may not be
the one the user is looking at.

## Decision

- **Per repository, in the repository:** each
  [repository](../glossary.md#repository) has a `.crannog/` directory, its
  [setup](../glossary.md#setup). There is no global configuration.

  ```
  .crannog/
    config.yml        # session, resources, proxy image, env, mounts
    iron-proxy.yaml   # native iron-proxy configuration
    container/        # build context
      Dockerfile
      ...             # files the Dockerfile copies
  ```

- **`config.yml`** also lists what crannog itself needs (proxy variables,
  CA, agent homes, caches), so nothing is hidden. Its schema is
  [ADR 0025](0025-configuration-schema.md).
- **`iron-proxy.yaml`** is iron-proxy's own format, without an abstraction
  on top. crannog relies on a few conventions in it
  ([setup](../features/setup.md)). Secret values never appear in the file.
- **Build context:** `container/` only. Other files in `.crannog/` never
  reach the image.
- **Source:** crannog reads `.crannog/` from the working tree, uncommitted
  changes included, only if the start branch is checked out in the current
  worktree. Otherwise it reads the start branch's latest commit. Without
  `.crannog/` there, it uses the preset `default`
  ([ADR 0026](0026-presets.md)). crannog always prints the source (e.g.
  `from feature/x (commit a1b2c3d)`, `from working tree (uncommitted
  changes)`, `from preset default`) and says so if uncommitted changes to
  `.crannog/` in the current worktree are not used.
- **Only an approved setup is used**
  ([ADR 0024](0024-setup-approval.md)): the image is built from it, the
  configuration is read from it, and the proxy receives a copy of its
  `iron-proxy.yaml`.

## Alternatives

- **Compose file:** not available with Apple Containers
  ([ADR 0008](0008-apple-containers-runtime.md)).
- **Global configuration in `~/.crannog/`:** not needed; per-repository
  configuration covers it and stays versioned.
- **TOML:** the user prefers YAML.
- **Wrapping iron-proxy's configuration in `config.yml`:** loses features and
  costs work on every iron-proxy update.
- **`.crannog/` as build context:** everything in `.crannog/` would reach the
  builder.
- **Mounting the project's `iron-proxy.yaml` into the proxy:** a merged change
  would take effect on the next reload without approval.
- **Reading the working tree of any worktree:** uses uncommitted changes the
  user does not see from where they are.

## Consequences

- The setup of a crannog is visible and versioned with the repository and can
  differ per branch.
- Users can break crannog by changing the values it relies on; nothing is
  enforced.
- Users who change `.crannog/` in one worktree and apply it to a branch
  checked out elsewhere must commit first.
