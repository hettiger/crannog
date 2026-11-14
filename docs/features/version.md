# version

- **Status:** Implemented
- **Use cases:** –
- **Decisions:** [0001](../decisions/0001-go-and-cobra.md), [0006](../decisions/0006-repository-layout.md)

## Summary

Prints the crannog version.

## Usage

```sh
crannog version
```

## Behavior

Prints the version string followed by a newline to stdout. The version is
set at build time via
`-ldflags "-X github.com/hettiger/crannog/crannog/cmd.version=..."`
(`make build` uses `git describe`). Without it, crannog prints the module
version Go recorded in the binary, which `go install` sets (a tag or a
pseudo-version such as `v0.0.0-20261108120000-738f3ce1a2b3`), and `dev`
for a plain `go build`.
