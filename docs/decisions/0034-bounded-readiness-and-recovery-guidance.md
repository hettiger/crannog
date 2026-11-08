# 0034. Bounded readiness and recovery guidance

- **Status:** Accepted
- **Date:** 2026-11-08

## Context

The readiness deadline in ADR 0024 did not interrupt a blocked runtime probe.
A failed container replacement could leave work volumes intact while checkout
suggested destroy. Destroy's loss report could also become stale while sessions
continued during confirmation.

## Decision

- Each proxy-health and readiness-marker wait has a 60-second context shared
  by its probes and status queries. At the deadline, send the runtime CLI
  process SIGTERM, which `container exec` forwards to the probe in the
  container (SIGKILL would leave the probe running), and wait for it to
  exit; after at most one additional second, kill it and stop draining
  inherited output pipes. This bounds the CLI, not a probe in the container
  that ignores SIGTERM. Reject success returned after cancellation.
  The proxy health probe also has its own `wget` timeout. This bounds
  readiness checks, not builds, starts or subsequent cleanup.
- When containers are missing and the selected setup requires a rebuild,
  checkout suggests `crannog apply` with that setup to recreate them.
  Otherwise it explains that the work volume may still hold work.
  Both messages explain that destroy deletes the volume. Recovery remains the
  existing apply behavior, not an automatic rollback.
- Describe a successful destroy check as a point-in-time result. If sessions
  run, explain that work created after the check is deleted too. Keep the
  existing confirmation and session-lifetime behavior.

## Alternatives

- Checking the deadline only between probes leaves blocking probes unbounded.
- Returning from a background probe without cancelling and waiting for the
  subprocess leaves runtime work running after the lifecycle lock is released.
- Transactional rebuilds or freezing all writers for destroy add coordination
  beyond the chosen scope of bounded waits and accurate recovery guidance.

## Consequences

Readiness probes cannot indefinitely delay a timeout. A failed rebuild can
still require manual recovery; unchanged or proxy-only setups do not recreate
missing containers. Destroy's report remains a snapshot rather than a guarantee
about work created before the user confirms.
