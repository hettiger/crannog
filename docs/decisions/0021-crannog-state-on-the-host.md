# 0021. Crannog state on the host

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog keeps state on the host: approved setups per repository, what
each crannog uses, locks and sessions, and the files the proxy and the
crannog mount. All git worktrees of a repository must share it. A build
can be interrupted at any point.

## Decision

- **Everything in `~/.crannog`:**

  ```
  ~/.crannog/
    agents/                     # agent homes (ADR 0011)
    cache/                      # package caches (ADR 0012)
    presets/<name>/             # user presets (ADR 0026)
    repositories/<id>/
      path                      # repository path, for humans and cleanup
      approved/<sha256>/        # approved setups, by setup hash (ADR 0024)
      approved/latest           # hash of the most recent approval
      crannogs/<slug>/          # one per crannog (ADR 0017)
        branch                  # the start branch (the slug is not reversible)
        crannog                 # hash of the approved setup it uses
        lock                    # see ADR 0019
        sessions/<pid>          # one per running session
        proxy/                  # mounted read-only into the proxy: CA, key,
                                # secrets/, iron-proxy.yaml
        ca/                     # ca.crt, for the setup to mount (CRANNOG_CA_DIR)
  ```

- **Repository id:** the sha256 in hex of the git common directory as
  `git rev-parse --path-format=absolute --git-common-dir` prints it, so all
  worktrees of a repository share it.
- **A crannog exists** once its `crannog` file holds a hash. A host directory
  without it is the remains of an interrupted build; `checkout` removes them
  and builds anew. If a first build fails, crannog removes everything it
  created (containers, network, volumes, remote, host state except the
  lock); the image stays.
- **Removal:** destroy removes `crannogs/<slug>/`; removing
  `repositories/<id>/` removes everything crannog knows about a repository.
- Access to these files follows
  [ADR 0013](0013-host-file-access.md).

## Alternatives

- **Leaving a failed build in place:** the user has to clean up or
  `destroy` a crannog that never worked.

## Consequences

- Moving a repository changes its `<id>`; crannog treats it as a new
  repository.
- Removing a repository from disk leaves its state under `~/.crannog/` until
  it is cleaned up.
- A crannog whose containers are missing while its host directory exists is
  an error that only `apply` or `destroy` resolves.
