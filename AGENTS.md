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
main.go                            CLI entry point (fang v2 CLI framework via internal/cmd)
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

- **`charm.land/fang/v2`**: Minimal CLI framework powering the command-line interface. Used in `internal/cmd/root.go` for command registration and execution.
- **`charm.land/fantasy`**: LLM provider abstraction layer. Handles protocol
  differences between Anthropic, OpenAI, Gemini, etc. Used in `internal/app`
  and `internal/agent`.
- **`charm.land/bubbles/v2`**: Reusable TUI components (textarea, textinput, filepicker, help, spinner, viewport, key). Used extensively in the UI layer.
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
  - **SQLC workflow**: Add queries to `internal/db/sql/*.sql`, then run `task sqlc`.
    Generated code includes interfaces (querier, models) in `internal/db/`.
  - **Migrations**: Numbered files in `internal/db/migrations/` (e.g., `20250514000000_*.sql`).
    Applied automatically on startup via `goose`. Never edit applied migrations.
  - **Query files**: One file per domain (`sessions.sql`, `messages.sql`, `files.sql`,
    `critic_reviews.sql`, etc.).
  - **sqlc.yaml config**: Schema from `internal/db/migrations`, queries from `internal/db/sql/`,
    emits interface, exact table names off, empty slices on.
- **Pub/sub**: `internal/pubsub` for decoupled communication between agent,
  UI, and services.
- **Hooks**: User-defined shell commands in `crush.json` that fire before
  tool execution. The engine (`internal/hooks/`) is independent of fantasy
  and agent — it takes inputs, runs commands, returns decisions. The
  `hookedTool` decorator in `internal/agent/hooked_tool.go` wraps tools at
  the coordinator level. Hooks run before permission checks. See
  `docs/hooks/README.md` for the user-facing protocol.
- **CGO disabled**: builds with `CGO_ENABLED=0` and
  `GOEXPERIMENT=greenteagc`.
- **Middleware pattern**: Skills wrap `SessionAgent` as decorators. The
  middleware receives a primary agent and config, exposes `SetXxx` wiring
  methods, and delegates all `SessionAgent` interface methods to the primary.
  Example:
  ```go
  type Middleware struct {
      primary agent.SessionAgent  // embedded as decorator
      cfg     SomeConfig
      // wiring fields set via SetXxx methods
  }
  func (m *Middleware) SetFileTracker(ft filetracker.Service) { m.filetracker = ft }
  func (m *Middleware) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
      // intercept Run(), then delegate to m.primary.Run()
  }
  // All other SessionAgent methods: delegate directly to m.primary
  ```
- **Skill auto-enable**: When a skill config section is present in crush.json
  (even with no fields), it auto-enables. Explicitly set `enabled: false` to
  disable.
- **Agent wrapper**: `AgentWrapper` func type in coordinator allows app layer to inject middleware without import cycles.
- **`composeWrappers` applies wrappers in list order**: first arg is innermost (wraps primary first), last arg is outermost. Critically: `buildCriticWrapper` wraps primary first (innermost), `buildReplacerWrapper` wraps that result, `buildToolcoachWrapper` wraps that result (outermost).
- **`SkipCoach` propagation**: The `SkipCoach(sessionID)` method propagates through the wrapper chain via interface type-assertion. Each wrapper (toolcoach, critic) delegates to its primary if the primary implements `interface{ SkipCoach(string) }`. Only the replacer (innermost) implements the actual skip logic (atomic flag + eval cancel). The coordinator type-asserts on `currentAgent` (outermost = toolcoach), so every wrapper in the chain must delegate for the signal to reach the replacer.
- **`critic` skill override config search paths**: When loading `.crush/skills/critic/config.json`, the code searches in order: `.crush`, `.kimi`, `crush` (first match wins). Not `.github/` or other common directories.
- **Hook `HaltExitCode`**: Exit code 49 halts the entire turn. It sits outside the generic-error range (1-30), sysexits range (64-78), and killed-by-signal range (128+) so it can't be hit accidentally.
- **Skill Tracker**: `skillTracker` in `internal/skills/tracker.go` tracks which
  skills have been mentioned in the conversation. Once a skill has been injected
  into the prompt (marked as "loaded"), it is not re-injected into subsequent
  prompts for the same session.
- **csync package**: Thread-safe wrappers (`csync.Value`, `csync.Map`, `csync.Slice`) avoid data races on agent state. Never use raw sync primitives for agent fields.

### Context Files

Crush reads context files from the working directory in priority order (first match wins):
```
.github/copilot-instructions.md  →  .cursorrules  →  .cursor/rules/  →
CLAUDE.md  →  CLAUDE.local.md  →  GEMINI.md  →  gemini.md  →
crush.md  →  crush.local.md  →  Crush.md  →  Crush.local.md  →
CRUSH.md  →  CRUSH.local.md  →  AGENTS.md  →  agents.md  →  Agents.md
```
Only the first matching file is loaded; remaining files with the same base name are skipped.

### Key UI Interaction Patterns

- **Textarea command interception**: In the chat textarea, certain strings are
  intercepted before they would be sent as prompts: `exit`/`quit` open the quit
  dialog; `/skipcoach` or `skipcoach` (without leading `/`) sends a skip signal
  to the replacer. These are handled in `internal/ui/model/ui.go` before the
  message-send path.
- **Commands dialog**: Pressing the key binding for the commands dialog (e.g. Ctrl+K)
  opens an overlay listing all available commands, including "Skip Coach" and "Toggle
  Yolo Mode". Implemented in `internal/ui/dialog/commands.go`.
- **Key bindings are in `model/keys.go`**: All keyboard shortcuts are defined there
  as `keyMap` fields on the UI model.

### Shell Builtins

Crush implements some shell builtins in Go for performance and portability:
- **jq**: Full jq syntax support via `gojq`. Available in both the Crush shell and the agent's Bash tool.
  - Usage: `jq <filter>` (reads from stdin or file arguments)
  - Example: `ls *.json | jq '.items[] | select(.active) | .name'`
- Builtins are intercepted in `internal/shell/run.go` via `builtinHandler()` middleware in the shell interpreter stack.

### Builtin Skills (`crush://skills/`)

Builtin skills are embedded in the binary at `builtin/` and use the virtual `crush://skills/` URL prefix:
- The prefix is NOT a URL, network address, or MCP resource — it is a special internal identifier.
- The View tool understands this prefix and reads from the embedded `builtinFS` filesystem.
- Builtin skills are discovered at startup via `fs.WalkDir(builtinFS, "builtin", ...)`.
- User skills can override builtin skills of the same name (last occurrence wins in discovery order).
- Current builtin skills: `crush-config`, `crush-hooks`, `jq`.

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
  - After adding new SQL queries to `internal/db/sql/*.sql`, run this command.
  - Generated code appears in `internal/db/` matching the package name.
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

Three middleware skills (wrappers):
- **`critic`**: After each agent edit, a secondary LLM reviews the diff and
  returns approve/revise/halt.
- **`replacer`**: Evaluates whether the conversation should continue with a
  follow-up prompt (conversation coach).
- **`toolcoach`**: Zero-LLM heuristic pattern detector that coaches the agent
  on anti-patterns (edit without view, destructive bash, missing multiedit, etc.)
  in real-time with sub-millisecond overhead.

Three built-in skills (prompt injectors via `crush://skills/` URL):
- **`crush-config`**: Configuration help
- **`crush-hooks`**: Hook authoring guide
- **`jq`**: jq syntax reference

### Middleware Stack Order

In `composeWrappers`, wrappers are applied in list order — first is innermost (closest to primary), last is outermost:
```
outermost: ToolcoachMiddleware
            ↓ wraps
           ReplacerMiddleware
            ↓ wraps
           CriticMiddleware
            ↓ wraps
inner-most: SessionAgent (primary)
```
This means toolcoach intercepts every tool call first (outermost), and the critic reviews the already-coached conversation.

The `skills` package implements the Agent Skills open standard
(https://agentskills.io). Skills are discovered from configured paths by
scanning for `SKILL.md` files with YAML frontmatter:

```go
skills.Discover(paths []string) []*Skill
// DiscoverBuiltin() reads from embedded builtin/ filesystem
// DiscoverBuiltinWithStates() returns per-file discovery state + errors
```

Discovery order: builtins first, then user-configured paths. When a user skill
has the same name as a builtin, the user skill takes precedence (last wins).

Each skill has a name, description, compatibility, and instructions. The
`ToPromptXML()` function generates XML for injection into system prompts.

Skills are enabled by:
1. Adding a config section in `crush.json` under `options` (auto-enables even if empty)
2. Setting `enabled: false` to explicitly disable

### Toolcoach Skill

A zero-LLM, heuristic-based skill that detects anti-patterns in the agent's
real-time tool usage and injects coaching tips into tool results. Overhead is
~2.7µs per tool call (benchmarked, well under 100µs target).

The toolcoach was built in four incremental phases:
- **Phase 0**: Telemetry & measurement infrastructure
- **Phase 1a/1b**: Core performance (zero-allocation, pre-compiled regex, precision heuristics)
- **Phase 2**: Critic integration (effectiveness DB, adaptive severity)
- **Phase 3**: Steering & active correction (progressive coaching, guided retry)

Each phase has a dedicated `PHASE*_IMPLEMENTATION.md` in `internal/skills/toolcoach/`.

**Anti-patterns detected**:

| ID | Tool | Severity | Trigger | Suggestion |
|----|------|----------|---------|------------|
| `destructive_bash` | `bash` | critical | `rm -rf /`, `rm -rf ~`, fork bombs | Use edit/write for safer changes |
| `write_over_existing` | `write` | warning | `write` to existing file | Consider edit to preserve content |
| `edit_without_view` | `edit` | hint | `edit` on file never viewed | View the file first |
| `repeated_view` | `view` | hint | Second+ view without edit | Use edit instead |
| `broad_grep` | `grep` | hint | Pattern < 3 chars or only wildcards | More specific pattern |
| `missing_multiedit` | `edit` | hint | 3+ consecutive edits to same file | Use multiedit to batch |

**Config** (in `crush.json` under `options.toolcoach`):
```json
{
  "toolcoach": {
    "enabled": true,
    "max_patterns_per_turn": 3,
    "enabled_patterns": []
  }
}
```

**Environment override**: `CRUSH_TOOLCOACH_DISABLED=1`
**Per-call disable**: Set `ToolcoachEnabled: &falseVar` in `SessionAgentCall`

Coaching tips include timing: `(coach delay: 42µs, spent: 150µs)` so users
can verify the overhead is negligible.

See `internal/skills/toolcoach/IMPLEMENTATION.md` for full details.

### Critic Skill

Reviews agent output across six dimensions: correctness, safety, idiomatics,
efficiency, testing, minimalism.

**Checkpoint flow**:
1. Middleware snapshots files via `filetracker` before agent runs.
2. After agent produces diff, `CriticService.Review()` submits to LLM.
3. LLM returns structured feedback: `approve`, `revise`, or `halt`.
4. On `revise`: rollback changes, inject feedback into conversation, re-drive agent.
5. On `halt`: rollback changes, return error to user.

**Wiring in `internal/app/critic.go`**:
- `buildCriticWrapper()` creates an `AgentWrapper` that injects the critic middleware.
- `buildReplacerWrapper()` creates the replacer wrapper.
- `buildToolcoachWrapper()` creates the toolcoach wrapper.
- `composeWrappers()` chains all three wrappers (toolcoach outermost, critic innermost).

**Critic–toolcoach integration**: The critic middleware implements the `CoachSummaryProvider` interface (`GetCoachSummary(sessionID) string`). The app wires `app.toolcoachMw` (an `atomic.Pointer[toolcoach.Middleware]`) as the critic's coach provider, so the critic can include real-time tool usage coaching summaries in its review context.

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

**Project overrides**: Place `.crush/skills/critic/config.json` or `.crush/skills/critic/prompt.md.tpl` in the working directory. The skill config loader searches `.crush/`, `.kimi/`, and `crush/` subdirectories (in that order, first match wins).

**Prompt injection defense**: User content (diff, plan) is wrapped in delimiters (`<<<DIFF_BEGIN>>>` / `<<<DIFF_END>>>`). The `escapeDelimiters()` function replaces any user text that looks like a delimiter with visually similar Unicode alternatives (`««»»`).

**Dual config structs**: The critic uses `critic.CriticSkillConfig` (in `internal/skills/critic/config.go`) internally, not `config.CriticConfig` (in `internal/config/config.go`). Both have the same fields, but `CriticSkillConfig` is the runtime struct used by the middleware. `NewCriticSkillConfig()` in `skillconfig.go` converts from `config.Config`.

### Replacer Skill

A conversation coach that decides whether the primary agent's response is
complete or needs a follow-up. Evaluates the full conversation history and
returns a `stop` or `continue` decision with an optional follow-up prompt.

**Config** (in `crush.json` under `options.replacer`):
```json
{
  "replacer": {
    "enabled": true,
    "model": "anthropic/claude-haiku-4",
    "max_iterations": 3,
    "timeout": "10s"
  }
}
```

The replacer auto-enables when the config section is present (if `enabled` is omitted it defaults to `true`; set `"enabled": false` to disable). It uses the small model by default if `model` is not specified.

**Environment variable**: `CRUSH_REPLACER_FORCE_CONTINUE=1` forces the replacer to continue in tests, bypassing model resolution.

**`/skipcoach` command**: Users can type `/skipcoach` (or `skipcoach` without the leading
  slash) in the textarea to interrupt the current evaluation without permanently
  disabling the replacer. This sets an atomic flag that `Run()` checks before and
  during evaluation, treating the interrupt as a user-initiated skip rather than an
  error. The skip signal propagates through the middleware chain:
  coordinator → toolcoach → replacer → critic → primary. It also appears as
  "Skip Coach" in the commands dialog (Ctrl+K or equivalent).

**Dual config structs**: Like the critic, the replacer uses `replacer.ReplacerConfig` (in `internal/skills/replacer/config.go`) internally, not `config.ReplacerConfig` (in `internal/config/config.go`). Both have the same fields; `NewReplacerConfig()` in `config.go` converts from `config.Config`.

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

Anytime you need to work on critic, replacer, or toolcoach skills, read the
corresponding documentation:
- `internal/skills/critic/SKILL.md` — user-facing feature docs
- `internal/skills/critic/IMPLEMENTATION.md` — detailed implementation
- `internal/skills/critic/TESTING.md` — test patterns and matrix
- `internal/skills/toolcoach/IMPLEMENTATION.md` — toolcoach implementation

Run skill tests with:
```bash
go test ./internal/skills/critic/... -v -race
go test ./internal/skills/replacer/... -v -race
go test ./internal/skills/toolcoach/... -v -race
```