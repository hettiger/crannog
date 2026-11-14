# crannog

Go CLI application built with [Cobra](https://github.com/spf13/cobra).
Module path: `github.com/hettiger/crannog`.

## Commands

All relevant commands are in the `Makefile`:

- Build: `make build` (macOS/arm64 binaries in `dist/`, version from
  `git describe`)
- Check: `make check` (`gofmt -l .` must report nothing, `go vet`, `go test`)
- Format: `make fmt`
- Third-party licenses: `make THIRD_PARTY_NOTICES`

## Layout

- `crannog/` – the `crannog` binary (package `main`): entrypoint only
- `crannog/cmd/` – Cobra commands (package `cmd`), one file per command,
  registered in `init()`
- `git-remote-crannog/` – the git remote helper binary (package `main`)
- `internal/` – packages shared by the binaries; the bundled presets are in
  `internal/preset/bundled/`
- `e2e/` – end-to-end smoke test, run on a Mac with Apple Containers
- `docs/` – project knowledge base (see below)

## Conventions

- Host paths, absolute or within an `os.Root`, are built with
  `filepath.Join`. Paths in a container, Git paths, `io/fs` paths and the
  file paths of a setup use `path`. Never concatenate `"/"`.
- Validate names from outside (branch slugs, setup hashes, preset names)
  before joining them: `Join` cleans `..` away.

## Language

This is an open source project. Everything in the repository is written in
English: code, identifiers, comments, docs, commit messages, CLI output.

## Knowledge base

`docs/` is the long-term memory of this project. Its purpose is that nobody has
to repeat context across sessions. Start at `docs/README.md`.

### Reading

- Before working on a topic, read `docs/README.md`, `docs/glossary.md` and any
  docs related to the topic (decisions, features, use cases).
- Treat accepted decisions as binding. If a request conflicts with one, point
  that out and ask whether the decision should change.

### Glossary (`docs/glossary.md`)

- Use the glossary terms consistently in code, docs, and conversation.
- When a new domain term comes up, or a term is used ambiguously or differently
  from its glossary definition, propose a definition and agree on it with the
  user **before** adding or changing the entry.
- Identifiers in code should follow the glossary terms.

### Decisions (`docs/decisions/`)

- Record every significant decision as soon as it is made, without being
  asked: technology choices, architecture, conventions, scope, naming,
  rejected alternatives worth remembering.
- One topic per ADR, in its current state. ADRs hold decisions, their
  reasons and alternatives; behavior, formats and implementation details
  belong in feature docs or the code.
- Until the first release, revise ADRs in place: edit the ADR a decision
  belongs to, or merge, split or renumber ADRs to keep them atomic. Update
  every reference (docs, spikes, code comments) in the same change. After
  the first release, never rewrite an accepted ADR's decision: supersede it
  with a new ADR and set the old one's status to `Superseded by NNNN`.
- New ADRs use `docs/decisions/template.md` and the next number
  (`NNNN-short-title.md`). Keep the index in `docs/decisions/README.md`
  current: grouped by topic, one line on what each ADR decides.

### Features (`docs/features/`) and use cases (`docs/use-cases/`)

- Document a use case when the user describes what someone wants to achieve
  with crannog. Document a feature when it is agreed on or implemented.
- Keep feature docs in sync with the code: status, commands, flags, behavior.
- Link features to the use cases they serve and the decisions behind them.

### Maintenance

- Update the knowledge base in the same change as the related code.
- Briefly tell the user which knowledge base files were added or changed.
- Keep docs concise. Prefer updating an existing doc over creating a new one.
- Only document what has been decided or implemented. Do not record open
  questions, options under evaluation, or speculative plans.
