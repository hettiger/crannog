# 0021. Crannog state on the host

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

crannog keeps state on the host: approved setups per repository, what
each crannog uses, locks and sessions, and the files the proxy and the
crannog mount. All git worktrees of a repository must share it. A build
can be interrupted at any point.

## Decision

- **Everything in `~/.crannog`**, created readable by the user only
  (mode 0700):

  ```
  ~/.crannog/
    agents/                     # agent homes (ADR 0011)
    cache/                      # package caches (ADR 0012)
    presets/<name>/             # user presets (ADR 0026)
    repositories/<id>/
      path                      # git common dir, for humans and cleanup
      approved/<sha256>/        # approved setups, by setup hash (ADR 0024)
      approved/latest           # hash of the most recent approval
      crannogs/<slug>/          # one per crannog (ADR 0017)
        branch                  # the start branch (the slug is not reversible)
        crannog                 # hash of the approved setup it uses
        config-checksum         # checksum of the interpolated config.yml
                                # its containers were created with
        agents-dir-mount        # where the agent homes are mounted
                                # writable in the crannog, or empty
        lock                    # see ADR 0019
        sessions/<pid>          # one per running session
        proxy/                  # mounted read-only into the proxy: CA, key,
                                # secrets/, iron-proxy.yaml
        ca/                     # ca.crt, for the setup to mount (CRANNOG_CA_DIR)
  ```

- **Repository id:** the sha256 in hex of the git common directory as
  `git rev-parse --path-format=absolute --git-common-dir` prints it, so all
  worktrees of a repository share it. `checkout` records that directory in
  `path`, since the id cannot be reversed.
- **A crannog exists** once its `crannog` file holds a hash. A host
  directory with a `branch` but no hash is the remains of an interrupted
  build; `checkout` removes them, whichever runtime objects exist, and
  builds anew. A directory with neither
  is new, and nothing is removed. If a first build fails, crannog removes
  everything it created (containers, network, volumes, remote, host
  directory); the image stays. If a removal fails, the host state
  stays so the next `checkout` tries again.
- **A first `checkout` that builds nothing leaves no host directory:** a
  declined approval or a failed build removes it, so a host directory
  always stands for a crannog or the remains of one.
- **Existing resources are refused:** if containers, a network or volumes
  with a new crannog's names already exist, for example after its host
  directory was deleted by hand, `checkout` names them and stops. This is
  a state crannog did not create; it changes nothing and the user removes
  them.
- **Missing runtime objects of an existing crannog:** a host directory
  with a hash says that crannog created the objects with its names, so
  crannog deals with whatever of them is gone, for example after a cleanup
  with the container CLI or a failed rebuild. Containers, the network and
  the state volume are replaceable: `checkout` and `apply` say what is
  missing and recreate it from the crannog's setup with a rebuild
  ([ADR 0022](0022-rebuilds.md)). The work volume is not: without it the
  repository in the crannog is gone, `checkout` and `apply` stop and point
  to `destroy`.
- **Removal:** destroy removes `crannogs/<slug>/`; removing
  `repositories/<id>/` removes everything crannog knows about a repository.
- Access to these files follows
  [ADR 0013](0013-host-file-access.md).

## Alternatives

- **Leaving a failed build in place:** the user has to clean up or
  `destroy` a crannog that never worked.
- **Deleting existing resources in `checkout`, with or without asking:**
  a second place besides `destroy` that deletes work.
- **Letting `destroy` remove existing resources without a host
  directory:** more code for a state only manual changes to `~/.crannog`
  produce.
- **Asking the user to choose between repairing, rebuilding from scratch
  and stopping when objects are missing:** crannog knows no better than
  the user whether a kept volume is intact, building from scratch is
  `destroy` followed by `checkout`, and a menu needs an answer without a
  terminal.
- **Missing containers as an error that only `apply` with a changed setup
  or `destroy` resolves:** a failed rebuild followed by reverting
  `.crannog/` left no way to keep the work volume.
- **Transferring the repository again into a new work volume:** that is a
  new crannog under the old name; the work in the old volume is gone
  either way.
- **Removing only what the failed build created:** keeps the existing
  resources, but the build still fails on the first name in use, with the
  runtime's error.

## Consequences

- Moving a repository changes its `<id>`; crannog treats it as a new
  repository.
- Removing a repository from disk leaves its state under `~/.crannog/` until
  it is cleaned up.
- A recreated state volume is empty: agents lose the state they kept in
  the crannog, and crannog says so.
- crannog does not check the contents of a work volume it keeps.
