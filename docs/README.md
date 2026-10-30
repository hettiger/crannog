# crannog knowledge base

This directory documents what crannog is, the terms we use, the decisions we
made, and the features and use cases it covers.

## Overview

crannog is a Go CLI that lets coding agents work autonomously inside sandboxes.

The name refers to the [crannog](https://en.wikipedia.org/wiki/Crannog), an
artificial island dwelling built in lakes, reachable only via a narrow
causeway. crannog builds such an island for an agent: an isolated place where it
can work safely. The only ways in and out are a few controlled connections
(git, optionally a network proxy, optionally volume mounts), and the human
controls all of them.

## Contents

| Path                               | Content                                   |
| ---------------------------------- | ----------------------------------------- |
| [glossary.md](glossary.md)         | Agreed domain terms and their meaning     |
| [decisions/](decisions/README.md)  | Architecture Decision Records (ADRs)      |
| [features/](features/README.md)    | Features, their status and behavior       |
| [use-cases/](use-cases/README.md)  | What users want to achieve with crannog   |

## Open questions

- Command names for building and destroying a crannog (the user will define
  these).
- Sandbox technology: containers (Docker/Podman), gVisor, microVMs, ...?
- Which coding agents are supported first?
- Git connection: how do changes get from the sandbox to the host, and how does
  the human review and accept them?
- Network proxy: default deny with allowlist?
