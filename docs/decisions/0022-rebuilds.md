# 0022. Rebuilds

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

When the setup a crannog uses changes, `checkout` and `apply` bring the
crannog up to date ([ADR 0027](0027-cli-commands.md)). A changed image,
environment, mount or resource only takes effect in a new container, which
ends the sessions running in the old one. A change to `iron-proxy.yaml`
takes effect with a reload of the proxy.

## Decision

- **What a rebuild replaces:** both containers, the crannog and its proxy,
  so changes to `proxy.image`, `env`, `mounts` and `resources` take effect.
  The volumes, the network, the CA, the remote's name and the host directory
  stay.
- **Image first:** crannog builds the new image before it replaces the
  containers. A failing build leaves the old containers in place, so
  `destroy` can still fetch from them.
- **Only `iron-proxy.yaml` changed:** crannog writes the proxy's copy and
  reloads the proxy; no session ends and nothing is asked beyond the setup
  approval. If the proxy is stopped, crannog only writes the copy.
- **`apply` never starts a crannog:** a rebuild by `apply` creates the new
  containers stopped; the next session starts them.
- **Running sessions:** if a rebuild would end running sessions, crannog
  shows the diff to the crannog's setup and asks for confirmation. `--yes`
  answers it. Without running sessions, a rebuild to an approved setup needs
  no question; crannog prints the setup's source.
- **One question:** if a setup approval is needed as well
  ([ADR 0024](0024-setup-approval.md)), the approval says that running
  sessions end. In `checkout`, the question replaces the confirmation for
  starting a session while another one runs.

## Alternatives

- **Rebuilding only the crannog's container:** a changed proxy image would
  only take effect after `destroy`.
- **Starting the crannog in `apply` to check that it boots:** costs a VM
  start; the next session reports a broken setup anyway.
- **Only warning that running sessions end:** sessions in other terminals
  end without the user agreeing.
- **Treating the question as an approval:** scripts could not rebuild a
  crannog with running sessions even with `--yes`, although nothing new is
  used.

## Consequences

- Running sessions only end after the user agreed to it or passed `--yes`.
- `checkout` asks at most one question before the build.
- `apply` with running sessions ends them after the confirmation; the
  crannog then stays stopped until the next session.
