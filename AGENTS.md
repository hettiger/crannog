# crannog

Go CLI application built with [Cobra](https://github.com/spf13/cobra).
Module path: `github.com/hettiger/crannog`.

## Commands

- Build: `go build ./...`
- Vet: `go vet ./...`
- Format: `gofmt -l .` must report nothing
- Test: `go test ./...`

## Layout

- `main.go` – entrypoint, calls `cmd.Execute()`
- `cmd/` – Cobra commands, one file per command, registered in `init()`
- `docs/` – project knowledge base (see below)

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
  that out and ask whether the decision should be superseded.

### Glossary (`docs/glossary.md`)

- Use the glossary terms consistently in code, docs, and conversation.
- When a new domain term comes up, or a term is used ambiguously or differently
  from its glossary definition, propose a definition and agree on it with the
  user **before** adding or changing the entry.
- Identifiers in code should follow the glossary terms.

### Decisions (`docs/decisions/`)

- Record every significant decision as an ADR as soon as it is made, without
  being asked: technology choices, architecture, conventions, scope, naming,
  rejected alternatives worth remembering.
- Use `docs/decisions/template.md`. Number sequentially (`NNNN-short-title.md`)
  and add the ADR to the index in `docs/decisions/README.md`.
- Never rewrite an accepted ADR's decision. Supersede it with a new ADR and
  update the old one's status to `Superseded by NNNN`.

### Features (`docs/features/`) and use cases (`docs/use-cases/`)

- Document a use case when the user describes what someone wants to achieve
  with crannog. Document a feature when it is agreed on or implemented.
- Keep feature docs in sync with the code: status, commands, flags, behavior.
- Link features to the use cases they serve and the decisions behind them.

### Maintenance

- Update the knowledge base in the same change as the related code.
- Briefly tell the user which knowledge base files were added or changed.
- Keep docs concise. Prefer updating an existing doc over creating a new one.
- Open questions go to `docs/README.md` under "Open questions" until resolved.
