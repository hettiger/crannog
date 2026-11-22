# 0029. Managed instructions block

- **Status:** Accepted
- **Date:** 2026-11-01

## Context

An agent in a crannog needs to know where it is: results leave only as
commits, `origin` has no URL, the network is limited to the proxy's
allowlist, `sudo` is available, and `.crannog/` defines the crannog. Agents
read global instructions from a file in their home, which lives in the
shared agent home ([ADR 0011](0011-agent-home.md)) and also holds the
user's own instructions. The host must not write there
([ADR 0014](0014-host-stays-out-of-guest-writable-directories.md)).

## Decision

- **A marked block:** crannog keeps a
  [managed instructions block](../glossary.md#managed-instructions-block)
  (`<!-- crannog:begin -->` … `<!-- crannog:end -->`) in each supported
  agent's global instruction file in its agent home (`CLAUDE.md`,
  `AGENTS.md`, [ADR 0028](0028-agents-in-the-default-image.md)). It explains
  the crannog: commit to return results, `origin` has no URL, network only
  via the proxy's allowlist, `sudo` is available, how `.crannog/` works.
- **Updated before every session:** crannog replaces the block to match its
  version, adds it if missing, and leaves the rest of the file alone.
- **Written by the crannog:** crannog reads each instruction file with
  `container exec` as the agent user, updates the block on the host in
  memory, and has the crannog write the file back only if it changed. The
  crannog creates missing agent homes.
- Without a writable mount of the agent homes in `config.yml`, no block is
  written.

## Alternatives

- **Instructions written once without updates:** become outdated when
  crannog changes.
- **Writing the block from the host on every build:** needs host access to
  the agent homes, and a crannog stays on an outdated block until its next
  rebuild.

## Consequences

- Changing the content of the block by hand has no lasting effect.
- Each session start runs one more `container exec` per agent, two if the
  block changes.
