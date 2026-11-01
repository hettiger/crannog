# version

- **Status:** Implemented
- **Use cases:** –
- **Decisions:** [0001](../decisions/0001-go-and-cobra.md), [0027](../decisions/0027-repository-layout.md), [0028](../decisions/0028-cobra-commands-in-a-subpackage.md)

## Summary

Prints the crannog version.

## Usage

```sh
crannog version
```

## Behavior

Prints the version string followed by a newline to stdout. The version is set
at build time via
`-ldflags "-X github.com/hettiger/crannog/crannog/cmd.version=..."` and
defaults to `dev`.
