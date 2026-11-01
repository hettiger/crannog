# 0024. Readiness marker

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

When a crannog starts, its entrypoint prepares it before a session can run:
it trusts the crannog's CA, takes ownership of the volumes and creates
directories in `/state` ([ADR 0013](0013-agents-default-image.md),
[ADR 0022](0022-default-image-packages-and-state.md)). crannog must wait for
that. The entrypoint of the spikes created `/tmp/ready` for this. The
container's writable layer survives stop and start
([ADR 0010](0010-git-causeway.md)), so after a restart that file exists
before the entrypoint has run.

## Decision

- **Contract:** the image's entrypoint writes the kernel's boot id to
  `/tmp/ready` once the crannog is prepared, before it idles:

  ```sh
  cat /proc/sys/kernel/random/boot_id > /tmp/ready
  ```

- **Waiting:** after starting a crannog, crannog waits until `/tmp/ready`
  holds the current content of `/proc/sys/kernel/random/boot_id`. Every
  start of the VM gives a new boot id, so a marker of an earlier start does
  not count.
- **Timeout:** after 60 seconds crannog stops waiting and fails with an
  error that names the marker, so users of their own Dockerfile know what
  their entrypoint must do.
- The `default` preset's entrypoint follows the contract. Images in
  `.crannog/container/` must as well.

## Alternatives

- **Removing the marker on start:** the entrypoint cannot remove it before
  crannog checks, and crannog cannot reach a stopped crannog's file system.
- **A marker in a tmpfs:** unverified whether Apple Containers provides one,
  and it would depend on the image.
- **No marker, retrying the session's command:** a session could start
  before the CA is trusted.

## Consequences

- A custom image without the marker fails after 60 seconds with a clear
  error instead of starting sessions in an unprepared crannog.
- The entrypoint's preparation runs on every start and must be repeatable.
