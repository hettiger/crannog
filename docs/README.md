# crannog knowledge base

This directory documents what crannog is, the terms we use, the decisions we
made, and the features and use cases it covers.

## Overview

crannog is a Go CLI that lets coding agents work autonomously inside sandboxes.

The name refers to the [crannog](https://en.wikipedia.org/wiki/Crannog), an
artificial island dwelling built in lakes, reachable only via a narrow
causeway. crannog builds such an island for an agent: an isolated place where it
can work safely. The only ways in and out are a few controlled connections
(git, a network proxy, optionally volume mounts), and the human
controls all of them.

Each crannog runs in its own lightweight VM via Apple Containers, so crannog
requires macOS 26 or later on Apple Silicon
([ADR 0008](decisions/0008-apple-containers-runtime.md)). Network access goes
through iron-proxy, which also injects credentials so they never enter the
crannog ([ADR 0009](decisions/0009-iron-proxy-proxy-causeway.md)). Agents keep
their logins in a shared agent home
([ADR 0011](decisions/0011-agent-home.md)). Code enters and leaves a crannog
through plain git remotes, via a remote helper over `container exec`; crannog
does not touch the user's SSH configuration
([ADR 0015](decisions/0015-git-causeway.md)).

Each repository configures its crannogs in `.crannog/` (image, environment,
mounts, proxy) ([ADR 0023](decisions/0023-repository-configuration.md)),
and crannog only uses a version the user has approved
([ADR 0024](decisions/0024-setup-approval.md)); without one, a preset is
used ([ADR 0026](decisions/0026-presets.md)). Crannogs keep the
repository on a `/work` volume and agent state on a `/state` volume
([ADR 0018](decisions/0018-volumes-and-repository-path.md)). A repository has
one crannog per start branch
([ADR 0017](decisions/0017-one-crannog-per-start-branch.md)), which runs
only while a session does ([ADR 0019](decisions/0019-sessions-and-auto-stop.md)). The default
image ships Claude Code, Codex, OpenCode and Pi, which share their logins and
settings across crannogs and can call each other
([ADR 0028](decisions/0028-agents-in-the-default-image.md)).

Day to day, `crannog checkout [branch]` opens a session in the crannog of a
branch, building or updating it as needed
([ADR 0027](decisions/0027-cli-commands.md)).

## Contents

| Path                               | Content                                   |
| ---------------------------------- | ----------------------------------------- |
| [glossary.md](glossary.md)         | Agreed domain terms and their meaning     |
| [decisions/](decisions/README.md)  | Architecture Decision Records (ADRs)      |
| [features/](features/README.md)    | Features, their status and behavior       |
| [use-cases/](use-cases/README.md)  | What users want to achieve with crannog   |
| [spikes/](spikes/README.md)        | Experiments that confirmed ADRs           |
