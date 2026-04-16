---
name: builtin-skills
description: Guide for adding and modifying builtin skills embedded in the Crush binary. Use when creating a new builtin skill, editing an existing one, or understanding how the embedded skill system works (internal/skills/builtin/).
---

# Builtin Skills

Crush embeds skills directly into the binary via `internal/skills/builtin/`.
These are always available without user configuration.

## How It Works

- Each skill lives in `internal/skills/builtin/<skill-name>/SKILL.md`.
- The tree is embedded at compile time via `//go:embed builtin/*` in
  `internal/skills/embed.go`.
- `DiscoverBuiltin()` walks the embedded FS, parses each `SKILL.md`, and
  sets paths with the `crush://skills/` prefix (e.g.,
  `crush://skills/jq/SKILL.md`).
- The View tool resolves `crush://` paths from the embedded FS, not disk.
- User skills with the same name override builtins (last occurrence wins
  in `Deduplicate()`).

## Adding a New Builtin Skill

1. Create `internal/skills/builtin/<skill-name>/SKILL.md` with YAML
   frontmatter (`name`, `description`) and markdown instructions. The
   directory name must match the `name` field.
2. No extra wiring needed — `//go:embed builtin/*` picks up new
   directories automatically.
3. Add a test assertion in `TestDiscoverBuiltin` in
   `internal/skills/skills_test.go` to verify discovery.
4. Build and test: `go build . && go test ./internal/skills/...`

## Existing Builtin Skills

| Skill | Directory | Description |
|-------|-----------|-------------|
| `crush-config` | `builtin/crush-config/` | Crush configuration help |
| `jq` | `builtin/jq/` | jq JSON processor usage guide |
