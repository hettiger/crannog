# preset list

- **Status:** In progress (PoC, not yet verified on a Mac)
- **Use cases:** –
- **Decisions:** [0013](../decisions/0013-host-file-access.md), [0026](../decisions/0026-presets.md), [0027](../decisions/0027-cli-commands.md)

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
