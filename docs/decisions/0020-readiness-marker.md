# 0020. Readiness marker

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

When a crannog starts, its entrypoint prepares it before a session can run:
in the default image, it trusts the crannog's CA, takes ownership of the
volumes and creates directories in `/state`
([default preset](../features/default-preset.md)). crannog must wait for
that. The entrypoint of the spikes created `/tmp/ready` for this. The
container's writable layer survives stop and start
([ADR 0018](0018-volumes-and-repository-path.md)), so after a restart that
file exists before the entrypoint has run.

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
  their entrypoint must do. The same 60 seconds bound every other command
  crannog runs itself in a crannog or against the runtime: the scripts
  before a session, the destroy check, the proxy reload, `container list`.
  Image builds, pulls, the repository transfer and sessions have no
  timeout; their duration depends on the setup and the user.
- The `default` preset's entrypoint follows the contract. Images in
  `.crannog/container/` must as well.

## Alternatives

- **Removing the marker on start:** the entrypoint cannot remove it before
  crannog checks, and crannog cannot reach a stopped crannog's file system.
- **A marker in a tmpfs:** unverified whether Apple Containers provides one,
  and it would depend on the image.
- **No marker, retrying the session's command:** a session could start
  before the CA is trusted.
- **No timeout at all:** a command that never returns is rare, and the
  user can press Ctrl-C. But without the deadline an entrypoint that never
  writes the marker makes `checkout` wait silently, and the error naming
  the marker is the one hint a user of a custom image gets.

## Consequences

- A custom image without the marker fails after 60 seconds with a clear
  error instead of starting sessions in an unprepared crannog.
- The entrypoint's preparation runs on every start and must be repeatable.
