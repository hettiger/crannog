# version

- **Status:** Implemented
- **Use cases:** –
- **Decisions:** [0001](../decisions/0001-go-and-cobra.md)

## Summary

Prints the crannog version.

## Usage

```sh
crannog version
```

## Behavior

Prints the version string followed by a newline to stdout. The version is set
at build time via
`-ldflags "-X github.com/hettiger/crannog/cmd.version=..."` and defaults to `dev`.
