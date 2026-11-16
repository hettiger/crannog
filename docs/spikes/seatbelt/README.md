# Spike: Seatbelt

Evidence for [ADR 0030](../../decisions/0030-seatbelt.md). Rerun it after
updating macOS or Apple Containers.

The spike runs commands through `/usr/bin/sandbox-exec` with the rules of
crannog's profile and checks what they can and cannot do.

## What it checks

1. **The profile:** reads everywhere; writes, deletes, renames and mode
   changes only in the allowed places, also through a symlink and from a
   child of a child; parameters must be resolved paths.
2. **The self-test:** creating a file in the home directory fails with
   "Operation not permitted" in the sandbox, and with another error
   outside of it if the file exists.
3. **Nesting:** which profiles can be entered from inside the sandbox.
4. **The setup directory:** `.crannog` can be replaced through
   `.crannog.tmp-*`; nothing else in the worktree can be written.
5. **git:** `status`, `remote`, `update-ref`, `ls-files`, `diff --no-index`
   with and without a pager and `commit`, in a linked worktree too.
6. **The `container` CLI:** build, volume, network, create, start, exec
   with a pipe and with a terminal, stop and delete; a guest writing into
   mounts.
7. **`go test`** (with `--go-test` only): crannog's tests with only the
   temporary directory and the build cache writable.

## Running

Requires Apple Silicon, macOS 26, the `container` CLI and internet access
for the image build. Run it in a terminal: some checks need one.

```bash
./spike.sh             # automated checks, cleans up afterwards
./spike.sh --keep      # keeps the work dir for inspection
./spike.sh --go-test   # also runs crannog's tests on the host, if all
                       # checks before passed
```

## Result

2026-11-14, macOS 27.2.1, `container` 1.6.1, Apple Git 2.54.0, Go 1.27.3,
with `--go-test`: **63 passed, 0 failed.**

- `go test ./...` and `go vet ./...` pass with only the temporary
  directory and the build cache writable, once the modules are in the
  module cache; `go` cannot download them in the sandbox. The Go
  telemetry counters in `~/Library/Application Support/go/telemetry`
  cannot be written, which the tools ignore.
- Seatbelt compares resolved paths: a parameter with a symlink in it
  allows nothing.
- Its regular expressions have no repetition counts such as `{8}`; the
  profile loads, but such a rule matches nothing.
- From inside the sandbox, `sandbox-exec` works only with the identical
  profile and parameters. Any other profile, wider or narrower, fails
  with `sandbox_apply: Operation not permitted`.
- `container build` writes the build context to
  `~/Library/Application Support/com.apple.container/builder` and an
  archive `<UUID>.tar` directly into the user's temporary directory. It
  ignores `$TMPDIR` and removes the archive afterwards. The other
  `container` commands write nothing themselves.
- A guest can write into a mount outside of the allowed places: the
  runtime's daemon is not in the sandbox.
- `rm` asks before deleting a file it cannot write to when stdin is a
  terminal.
- The system log does not show the writes denied to the `container` CLI;
  its error messages do.
