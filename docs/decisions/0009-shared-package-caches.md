# 0009. Shared package manager caches

- **Status:** Accepted
- **Date:** 2026-10-31

## Context

Agents install dependencies with package managers (npm, pnpm, Composer, ...)
and download tools such as Playwright browsers. Without a cache that outlives a
crannog, every crannog and every recreation of a crannog's container downloads
everything again.

Package manager caches are additive and content-addressed: entries are written
to a temporary file and renamed into place, and a broken entry fails the
integrity check and is downloaded again. Several processes can use them at the
same time.

## Decision

- Package manager and tool caches live on the host under
  `~/.crannog/cache/<tool>` and are shared by all crannogs.
- `~/.crannog/cache` is bind-mounted into every crannog at the same path in the
  agent user's home, like the agent home ([ADR 0008](0008-agent-home.md)).
  Environment variables point the tools there (e.g. `NPM_CONFIG_CACHE`,
  `COMPOSER_CACHE_DIR`, `PLAYWRIGHT_BROWSERS_PATH`).
- `PLAYWRIGHT_BROWSERS_PATH` is combined with `PLAYWRIGHT_SKIP_BROWSER_GC=1`,
  so one crannog does not remove browsers another crannog uses.
- Project dependencies (`node_modules`, `vendor`) are not shared. They stay in
  the repository inside the crannog.

## Alternatives

- **Caches per crannog:** no cross-crannog risk, but every crannog downloads
  everything again.
- **Installing tools in the image:** works for tools such as Playwright
  browsers, but their version must match the project's, and the image grows.

## Consequences

- Dependencies are downloaded once per Mac, not once per crannog.
- pnpm cannot hardlink from a store on another file system and copies instead,
  which makes installs slower.
- Cleanup commands (e.g. `npm cache clean`) affect all crannogs.
- A compromised agent can poison the shared caches, e.g. replace a Playwright
  browser binary or a cached Composer archive without a checksum in the lock
  file. The code then runs in other crannogs. This risk is accepted, like the
  shared agent home in ADR 0008.
