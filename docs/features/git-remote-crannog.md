# git-remote-crannog

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** [Let a coding agent work autonomously](../use-cases/autonomous-agent.md)
- **Decisions:** [0010](../decisions/0010-git-causeway.md), [0012](../decisions/0012-crannog-names-volumes-paths.md), [0014](../decisions/0014-one-crannog-per-start-branch.md), [0027](../decisions/0027-repository-layout.md), [0032](../decisions/0032-anchored-host-file-access.md)

## Summary

The git remote helper of the git causeway. git runs it for remotes with the
URL `crannog::<slug>`.

## Usage

Installed next to `crannog` in `PATH`; users never call it. Any git command
that works with a remote works with a running crannog's remote:

```sh
git fetch crannog-<slug>
git push crannog-<slug> main
```

## Behavior

- A URL whose slug is not a branch slug is an error.
- Supports only the `connect` capability. On `connect git-upload-pack` or
  `connect git-receive-pack`, it replaces itself with
  `container exec -i --user agent <crannog> <service> /work/<host path>`,
  passing `GIT_PROTOCOL` if git sets it.
- Finds the repository from git's environment (`GIT_DIR`), so the
  container name and the repository path in the crannog follow from the
  repository and the slug.
- Never starts a crannog: if it is not running, it fails with a hint to
  `crannog checkout`.
