# Decisions

Architecture Decision Records. New ADRs use [template.md](template.md).

| ADR                                              | Title                                                    | Status                                             |
| ------------------------------------------------ | -------------------------------------------------------- | -------------------------------------------------- |
| [0001](0001-go-and-cobra.md)                     | Go and Cobra for the CLI                                 | Accepted, partially superseded by 0027             |
| [0002](0002-mit-license.md)                      | MIT license                                              | Accepted                                           |
| [0003](0003-english-only.md)                     | English-only repository                                  | Accepted                                           |
| [0004](0004-knowledge-base.md)                   | Knowledge base in `docs/`                                | Accepted                                           |
| [0005](0005-domain-language.md)                  | Domain language                                          | Accepted                                           |
| [0006](0006-apple-containers-runtime.md)         | Apple Containers as sandbox runtime                      | Accepted                                           |
| [0007](0007-iron-proxy-proxy-causeway.md)        | iron-proxy as proxy causeway                             | Accepted, partially superseded by 0020             |
| [0008](0008-agent-home.md)                       | Agent logins in a shared agent home                      | Accepted, partially superseded by 0013             |
| [0009](0009-shared-package-caches.md)            | Shared package manager caches                            | Accepted                                           |
| [0010](0010-git-causeway.md)                     | Git causeway via a remote helper                         | Accepted, partially superseded by 0012, 0014       |
| [0011](0011-repository-configuration.md)         | Repository configuration in `.crannog/`                  | Accepted, partially superseded by 0014, 0015, 0019 |
| [0012](0012-crannog-names-volumes-paths.md)      | Crannog names, volumes and paths                         | Accepted, partially superseded by 0014             |
| [0013](0013-agents-default-image.md)             | Agents in the default image                              | Accepted, partially superseded by 0022             |
| [0014](0014-one-crannog-per-start-branch.md)     | One crannog per start branch                             | Accepted, partially superseded by 0021, 0025       |
| [0015](0015-presets.md)                          | Presets                                                  | Accepted                                           |
| [0016](0016-cli-commands.md)                     | CLI commands                                             | Accepted, partially superseded by 0018             |
| [0017](0017-go-dependencies.md)                  | Go dependencies                                          | Accepted                                           |
| [0018](0018-rebuilds-with-running-sessions.md)   | Rebuilds with running sessions                           | Accepted                                           |
| [0019](0019-configuration-schema.md)             | Configuration schema and content hash                    | Accepted, partially superseded by 0020             |
| [0020](0020-proxy-address.md)                    | Proxy address in a crannog                               | Accepted                                           |
| [0021](0021-slugs-with-branch-hash.md)           | Slugs with a branch hash                                 | Accepted, partially superseded by 0026             |
| [0022](0022-default-image-packages-and-state.md) | Default image: system packages and agent state           | Accepted                                           |
| [0023](0023-git-author-from-the-host.md)         | Git author from the host                                 | Accepted                                           |
| [0024](0024-readiness-marker.md)                 | Readiness marker                                         | Accepted                                           |
| [0025](0025-session-locks-and-lifecycle.md)      | Session locks and lifecycle details                      | Accepted                                           |
| [0026](0026-empty-branch-slugs.md)               | Branch slugs of branches without ASCII letters or digits | Accepted                                           |
| [0027](0027-repository-layout.md)                | Repository layout with one directory per binary          | Accepted, partially superseded by 0028             |
| [0028](0028-cobra-commands-in-a-subpackage.md)   | Cobra commands in a subpackage                           | Accepted                                           |
| [0029](0029-poc-lifecycle-details.md)            | Lifecycle details of the PoC                             | Accepted                                           |
| [0030](0030-poc-implementation-conventions.md)   | Implementation conventions of the PoC                    | Proposed                                           |
