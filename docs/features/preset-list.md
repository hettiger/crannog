# preset list

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** –
- **Decisions:** [0015](../decisions/0015-presets.md), [0016](../decisions/0016-cli-commands.md), [0031](../decisions/0031-host-file-access.md)

## Summary

Lists all presets, bundled and user.

## Usage

```sh
crannog preset list
```

## Behavior

Prints a table of name, source (`bundled`, `user`, or `user, replaces
bundled`) and the directory of user presets in `~/.crannog/presets/`.
Hidden directories and symlinks there are not presets. Works outside a
repository.
