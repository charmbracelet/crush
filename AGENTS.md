# Crush Development Guide

## Project Overview

Crush is a terminal-based AI coding assistant built in Go by
[Charm](https://charm.land). It connects to LLMs and gives them tools to read,
write, and execute code. It supports multiple providers (Anthropic, OpenAI,
Gemini, Bedrock, Copilot, Hyper, MiniMax, Vercel, and more), integrates with
LSPs for code intelligence, and supports extensibility via MCP servers and
agent skills.

The module path is `github.com/charmbracelet/crush`.

## Architecture

```
main.go                            CLI entry point (cobra via internal/cmd)
internal/
  app/app.go                       Top-level wiring: DB, config, agents, LSP, MCP, events
  cmd/                             CLI commands (root, run, login, models, stats, sessions)
    critic-demo/                   Standalone demo binary for critic + replacer (no API keys)
  config/
    config.go                      Config struct, context file paths, agent definitions
    load.go                        crush.json loading and validation
    provider.go                    Provider configuration and model resolution
  agent/
    agent.go                       SessionAgent: runs LLM conversations per session
    coordinator.go                 Coordinator: manages named agents ("coder", "task")
    hooked_tool.go                 Decorator that runs PreToolUse hooks before tool execution
    prompts.go                     Loads Go-template system prompts
    templates/                     System prompt templates (coder.md.tpl, task.md.tpl, etc.)
    tools/                         All built-in tools (bash, edit, view, grep, glob, etc.)
      mcp/                         MCP client integration
  hooks/                           Hook engine: runs user shell commands on hook events
    hooks.go                       Decision types, aggregation logic, event constants
    runner.go                      Parallel hook execution, timeout, dedup
    input.go                       Stdin payload builder, env vars, stdout parsing (Crush + Claude Code compat)
  skills/
    skills.go                      Skills discovery and loading (Agent Skills open standard)
    critic/                        Self-critic middleware: reviews agent diffs, returns approve/revise/halt
      middleware.go                Core decorator wrapping SessionAgent.Run()
      service.go                   Review orchestration: cache, retry, pub/sub
      checkpoint.go                Types and gating logic
      config.go                    Runtime config with defaults
      prompt.go                    Template rendering with injection defense
      diff.go                      Git vs library diff with binary detection
      snapshot.go                  File snapshot / rollback with size limits
      cache.go                     SHA-256 keyed LRU cache
      diagnostics.go               LSP diagnostic fetching
      store.go                     SQLite persistence via sqlc
      breaker.go                   Circuit breaker for retryable errors
      parser.go                    JSON extraction with fallback strategies
      prompt_message_test.go       Tests for message checkpoint prompts
    replacer/                      Replacement agent middleware: conversation continuation coach
      middleware.go                Core decorator wrapping primary agent
      config.go                    Runtime config
      prompt.go                    Template for evaluation decisions
      parser.go                    Parses stop/continue decisions from LLM
  session/session.go               Session CRUD backed by SQLite
  message/                         Message model and content types
  db/                              SQLite via sqlc, with migrations
    sql/                           Raw SQL queries (consumed by sqlc)
    migrations/                    Schema migrations
  lsp/                             LSP client manager, auto-discovery, on-demand startup
  ui/                              Bubble Tea v2 TUI (see internal/ui/AGENTS.md)
  permission/                      Tool permission checking and allow-lists
  shell/                           Bash command execution with background job support
  event/                           Telemetry (PostHog)
  pubsub/                          Internal pub/sub for cross-component messaging
  filetracker/                     Tracks files touched per session
  history/                         Prompt history
```

### Key Dependency Roles

- **`charm.land/fantasy`**: LLM provider abstraction layer. Handles protocol
  differences between Anthropic, OpenAI, Gemini, etc. Used in `internal/app`
  and `internal/agent`.
- **`charm.land/bubbletea/v2`**: TUI framework powering the interactive UI.
- **`charm.land/lipgloss/v2`**: Terminal styling.
- **`charm.land/glamour/v2`**: Markdown rendering in the terminal.
- **`charm.land/catwalk`**: Snapshot/golden-file testing for TUI components.
- **`sqlc`**: Generates Go code from SQL queries in `internal/db/sql/`.

### Key Patterns

- **Config is a Service**: accessed via `config.Service`, not global state.
- **Tools are self-documenting**: each tool has a `.go` implementation and a
  `.md` description file in `internal/agent/tools/`.
- **System prompts are Go templates**: `internal/agent/templates/*.md.tpl`
  with runtime data injected.
- **Context files**: Crush reads AGENTS.md, CRUSH.md, CLAUDE.md, GEMINI.md
  (and `.local` variants) from the working directory for project-specific
  instructions.
- **Persistence**: SQLite + sqlc. All queries live in `internal/db/sql/`,
  generated code in `internal/db/`. Migrations in `internal/db/migrations/`.
- **Pub/sub**: `internal/pubsub` for decoupled communication between agent,
  UI, and services.
- **Hooks**: User-defined shell commands in `crush.json` that fire before
  tool execution. The engine (`internal/hooks/`) is independent of fantasy
  and agent — it takes inputs, runs commands, returns decisions. The
  `hookedTool` decorator in `internal/agent/hooked_tool.go` wraps tools at
  the coordinator level. Hooks run before permission checks. See
  `HOOKS.md` for the user-facing protocol.
- **CGO disabled**: builds with `CGO_ENABLED=0` and
  `GOEXPERIMENT=greenteagc`.
- **Middleware pattern**: Skills wrap `SessionAgent` as decorators. The
  middleware receives a primary agent and config, exposes `SetXxx` wiring
  methods, and delegates all `SessionAgent` interface methods to the primary.
- **Skill auto-enable**: When a skill config section is present in crush.json
  (even with no fields), it auto-enables. Explicitly set `enabled: false` to
  disable.
- **Agent wrapper**: `AgentWrapper` func type in coordinator allows app layer
  to inject middleware without import cycles.

## Build/Test/Lint Commands

- **Install linter** (first time only): `task lint:install`
- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test -race -failfast ./...`
- **Test specific package**: `go test ./internal/skills/critic/... -v`
- **Test replacer package**: `go test ./internal/skills/replacer/... -v`
- **Record VCR cassettes**: `task test:record` (re-records all agent VCR cassettes; takes ~1 hour)
- **Update Golden Files**: `go test ./... -update`
  - Update specific package:
    `go test ./internal/ui/components/core -update`
- **Lint**: `task lint` (runs `task lint:log` + `golangci-lint run`)
- **Lint log check**: `task lint:log` (checks that log messages start with capital letters)
- **Lint:fix**: `task lint:fix` (runs linters with auto-fix)
- **Format**: `task fmt` (`gofumpt -w .`)
- **Modernize**: `task modernize` (runs `modernize` which makes code
  simplifications)
- **Dev**: `task dev` (runs with profiling enabled)
- **SQLC generate**: `task sqlc` (regenerates Go code from SQL queries)
- **Schema generate**: `task schema` (regenerates JSON schema from config)
- **Update Hyper provider**: `task hyper` (updates embedded provider.json)
- **Update dependencies**: `task deps` (updates Fantasy and Catwalk)
- **Demo binary**: `go run ./cmd/critic-demo` (runs standalone critic + replacer
  demo with no API keys)

### Interactive TUI

```bash
crush run "prompt"          # Start a new chat session
crush login                  # Authenticate with providers
crush models                 # List available models
crush sessions               # Manage chat sessions
crush logs                   # View request/response logs
```

### Critic Inspection

```bash
crush critic list --session <id>        # List reviews for a session
crush critic show --message <id>         # Show a single review
crush critic stats                       # Aggregate statistics
```

### Data Directory

Use `--data-dir <path>` flag on critic commands to point to a specific crush data directory (defaults to `~/.crush`).

## Code Style Guidelines

- **Imports**: Use `goimports` formatting, group stdlib, external, internal
  packages.
- **Formatting**: Use gofumpt (stricter than gofmt), enabled in
  golangci-lint.
- **Naming**: Standard Go conventions — PascalCase for exported, camelCase
  for unexported.
- **Types**: Prefer explicit types, use type aliases for clarity (e.g.,
  `type AgentName string`).
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for
  wrapping.
- **Context**: Always pass `context.Context` as first parameter for
  operations.
- **Interfaces**: Define interfaces in consuming packages, keep them small
  and focused.
- **Structs**: Use struct embedding for composition, group related fields.
- **Constants**: Use typed constants with iota for enums, group in const
  blocks.
- **Testing**: Use testify's `require` package, parallel tests with
  `t.Parallel()`, `t.SetEnv()` to set environment variables. Always use
  `t.Tempdir()` when in need of a temporary directory. This directory does
  not need to be removed.
- **JSON tags**: Use snake_case for JSON field names.
- **File permissions**: Use octal notation (0o755, 0o644) for file
  permissions.
- **Log messages**: Log messages must start with a capital letter (e.g.,
  "Failed to save session" not "failed to save session").
  - This is enforced by `task lint:log` which runs as part of `task lint`.
- **Comments**: End comments in periods unless comments are at the end of the
  line.

## Testing with Mock Providers

When writing tests that involve provider configurations, use the mock
providers to avoid API calls:

```go
func TestYourFunction(t *testing.T) {
    originalUseMock := config.UseMockProviders
    config.UseMockProviders = true
    defer func() {
        config.UseMockProviders = originalUseMock
        config.ResetProviders()
    }()

    config.ResetProviders()
    providers := config.Providers()
    // ... test logic
}
```

## Skills System

### Overview

Skills are middleware decorators that wrap the primary `SessionAgent`. Each
skill implements the `agent.SessionAgent` interface by embedding the primary
agent and delegating all methods, while intercepting `Run()` to add behavior.

Two built-in skills:
- **`critic`**: After each agent edit, a secondary LLM reviews the diff and
  returns approve/revise/halt.
- **`replacer`**: Evaluates whether the conversation should continue with a
  follow-up prompt (conversation coach).

### Skill Discovery

The `skills` package implements the Agent Skills open standard
(https://agentskills.io). Skills are discovered from configured paths by
scanning for `SKILL.md` files with YAML frontmatter:

```go
skills.Discover(paths []string) []*Skill
```

Each skill has a name, description, compatibility, and instructions. The
`ToPromptXML()` function generates XML for injection into system prompts.

### Critic Skill

Reviews agent output across six dimensions: correctness, safety, idiomatics,
efficiency, testing, minimalism.

**Config** (in `crush.json` under `options.critic`):
```json
{
  "critic": {
    "enabled": true,
    "model": "anthropic/claude-sonnet-4",
    "max_iterations": 3,
    "auto_approve": false,
    "threshold": 0.85,
    "cache_size": 32,
    "max_diff_size": 32768,
    "max_file_size": 10485760,
    "timeout": "10s",
    "retention_days": 30
  }
}
```

**Environment overrides**: `CRUSH_CRITIC_DISABLED=1` (app-level disable, sets `Enabled=false` in config), `CRUSH_CRITIC_GLOBAL_DISABLE=1` (skill-level kill switch, forces `Enabled=false` in runtime config regardless of config), `CRUSH_CRITIC_MODEL`, `CRUSH_CRITIC_THRESHOLD`, `CRUSH_CRITIC_MAX_ITERATIONS`, `CRUSH_CRITIC_AUTO_APPROVE`, `CRUSH_CRITIC_TIMEOUT`, `CRUSH_CRITIC_MAX_DIFF_SIZE`, `CRUSH_CRITIC_MAX_FILE_SIZE`, `CRUSH_CRITIC_RETENTION_DAYS`.

**Per-session disable**: Set `CriticEnabled: &falseVar` in
`SessionAgentCall` (nil = use global, false = disable for this call).

**Project overrides**: Place `.crush/skills/critic/config.json` or
`.crush/skills/critic/prompt.md.tpl` in the working directory.

**Prompt injection defense**: User content (diff, plan) is wrapped in
delimiters (`<<<DIFF_BEGIN>>>` / `<<<DIFF_END>>>`). The
`escapeDelimiters()` function replaces any user text that looks like a
delimiter with visually similar Unicode alternatives (`««»»`).

### Replacer Skill

A conversation coach that decides whether the primary agent's response is
complete or needs a follow-up. Evaluates the full conversation history and
returns a `stop` or `continue` decision with an optional follow-up prompt.

**Config** (in `crush.json` under `options.replacer`):
```json
{
  "replacer": {
    "enabled": true,
    "max_iterations": 3
  }
}
```

The replacer auto-enables when the config section is present (if `enabled` is omitted it defaults to `true`; set `"enabled": false` to disable).

**Environment variable**: `CRUSH_REPLACER_FORCE_CONTINUE=1` forces the
replacer to continue in tests, bypassing model resolution.

## Formatting

- ALWAYS format any Go code you write.
  - First, try `gofumpt -w .`.
  - If `gofumpt` is not available, use `goimports`.
  - If `goimports` is not available, use `gofmt`.
  - You can also use `task fmt` to run `gofumpt -w .` on the entire project,
    as long as `gofumpt` is on the `PATH`.

## Comments

- Comments that live on their own lines should start with capital letters and
  end with periods. Wrap comments at 78 columns.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`,
  `docs:`, `sec:`, etc).
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.

## Provider Types

Supported provider types (used in `crush.json` providers `type` field):
- `openai` — OpenAI API
- `anthropic` — Anthropic API
- `openai-compat` — OpenAI-compatible REST API
- `openrouter` — OpenRouter
- `vercel` — Vercel AI SDK
- `azure` — Azure OpenAI
- `bedrock` — AWS Bedrock
- `gemini` — Google Gemini
- `google-vertex` — Google Vertex AI
- `hyper` — Charm Hyper

## Working on the TUI (UI)

Anytime you need to work on the TUI, read `internal/ui/AGENTS.md` before
starting work.

## Working on Skills

Anytime you need to work on the critic or replacer skills, read the
corresponding documentation:
- `internal/skills/critic/SKILL.md` — user-facing feature docs
- `internal/skills/critic/IMPLEMENTATION.md` — detailed implementation
- `internal/skills/critic/TESTING.md` — test patterns and matrix

Run skill tests with:
```bash
go test ./internal/skills/critic/... -v -race
go test ./internal/skills/replacer/... -v -race
```