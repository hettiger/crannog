# Features

One file per feature (`short-name.md`), listed here.

| Feature                                       | Status      | Use cases                                          | Summary                                         |
| --------------------------------------------- | ----------- | -------------------------------------------------- | ----------------------------------------------- |
| [apply](apply.md)                             | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | Apply changes to `.crannog/` without a session  |
| [checkout](checkout.md)                       | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | Open a session in the crannog of a branch       |
| [default preset](default-preset.md)           | In progress | [autonomous-agent](../use-cases/autonomous-agent.md) | The bundled `.crannog/` for any repository      |
| [destroy](destroy.md)                         | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | Destroy a crannog after showing what is lost    |
| [git-remote-crannog](git-remote-crannog.md)   | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | git remote helper of the git causeway           |
| [list](list.md)                               | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | List the repository's crannogs                  |
| [preset list](preset-list.md)                 | Implemented | –                                                  | List bundled and user presets                   |
| [preset publish](preset-publish.md)           | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | Write a preset to `.crannog/`                   |
| [setup](setup.md)                             | Implemented | [autonomous-agent](../use-cases/autonomous-agent.md) | Format of `.crannog/` and what crannog expects  |
| [version](version.md)                         | Implemented | –                                                  | Print the crannog version                       |

<!--
Feature file format:

# Feature name

- **Status:** Planned | In progress | Implemented | Deprecated
- **Use cases:** links to docs/use-cases/
- **Decisions:** links to docs/decisions/

## Summary
## Usage
## Behavior
-->
