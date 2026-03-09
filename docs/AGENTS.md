# Repository Guidelines

## Project Structure & Module Organization

This directory contains product and architecture documentation for Crush. Keep
documents in the folder that matches their lifecycle stage:

- `implemented/`: shipped behavior and current capabilities
- `active-design/`: in-progress designs and proposals
- `future/`: longer-term concepts
- `research/`: investigations and notes
- `adr/`: Architecture Decision Records
- `forReview/`: reviewer packets and `*-review.md` feedback files
- `archive/`: superseded material

Use descriptive kebab-case filenames such as `dispatch-api.md` or
`multi-agent-orchestration.md`.

## Build, Test, and Development Commands

This docs area has no standalone build step. Useful repository commands live at
the project root:

- `go build .`: build Crush
- `go test ./...`: run the full Go test suite
- `task test`: run the standard test workflow
- `task lint`: run lint checks
- `task fmt`: format Go code

For docs changes, verify links, paths, and command examples manually before
submitting.

## Coding Style & Naming Conventions

Write in concise, instructional Markdown. Prefer short sections, clear headings,
and bullet lists for scanability. Keep terminology consistent with the product:
use "session", "handler", "worker", and "dispatch" the same way they appear in
`README.md` and design docs.

Use:

- ATX headings (`#`, `##`, `###`)
- fenced code blocks for commands and directory trees
- relative paths like `forReview/README.md`

## Testing Guidelines

There is no docs-specific test harness in this directory. Treat review as a
quality gate:

- confirm examples match current repository commands
- make sure links resolve from the current file
- update adjacent docs when moving a feature between `active-design/`,
  `implemented/`, and `archive/`

## Commit & Pull Request Guidelines

Recent history uses semantic commit prefixes such as `fix(ui): ...`,
`fix(agent): ...`, `ci: ...`, and `chore(deps): ...`. Follow that pattern for
docs work, for example: `docs: clarify dispatch session lifecycle`.

Pull requests should include a short summary, the directories changed, and any
review context needed. When editing reviewer materials, mention the affected
file in `forReview/` and note whether follow-up updates are expected elsewhere.
