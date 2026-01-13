# Subagent Feature Proposal for Crush

## Overview

This proposal outlines adding custom subagent support to Crush, compatible with
Claude Code's agent feature. Subagents are specialized AI assistants that run in
isolated sessions with custom system prompts, specific tool access, and
configurable models.

## Goals

1. **Claude Code compatibility**: Support the same Markdown + YAML frontmatter
   format used by Claude Code (`~/.claude/agents/*.md`)
2. **Session isolation**: Subagents run in their own context window; only the
   final result returns to the parent
3. **Configurable models**: Allow subagents to use different models (e.g., Haiku
   for exploration, Opus for complex reasoning)
4. **Tool restrictions**: Whitelist/denylist tools per subagent
5. **Proactive delegation**: Main agent automatically delegates based on
   subagent descriptions

## Claude Code Format Reference

Claude Code subagents are defined as Markdown files with YAML frontmatter:

```yaml
---
name: code-reviewer
description: Reviews code for quality and best practices. Use proactively...
tools: Read, Glob, Grep, Bash
disallowedTools: Write, Edit
model: sonnet  # or: opus, haiku, inherit
permissionMode: default  # or: acceptEdits, dontAsk, bypassPermissions, plan
color: pink
---

You are a senior code reviewer...
```

**Frontmatter fields** (from Claude Code docs):

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier (lowercase, hyphens) |
| `description` | Yes | When to delegate to this subagent |
| `tools` | No | Allowlist of tools (inherits all if omitted) |
| `disallowedTools` | No | Denylist (removes from inherited/specified) |
| `model` | No | `sonnet`, `opus`, `haiku`, or `inherit` |
| `permissionMode` | No | Permission handling mode |
| `skills` | No | Skills to inject into system prompt |
| `hooks` | No | Lifecycle hooks (PreToolUse, PostToolUse, Stop) |
| `color` | No | Background color in UI |

**File locations** (priority order):

1. `--agents` CLI flag (JSON, session-only)
2. `.crush/agents/` (project-level, checked into VCS)
3. `~/.config/crush/agents/` (user-level, all projects)
4. Plugin agents (lowest priority)

## Current Crush Architecture

### Existing Agent Tool

Crush already has a basic subagent implementation in `internal/agent/agent_tool.go`:

```go
func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
    agentCfg := c.cfg.Agents[config.AgentTask]
    agent, err := c.buildAgent(ctx, prompt, agentCfg, true /* isSubAgent */)
    
    return fantasy.NewParallelAgentTool(AgentToolName, description,
        func(ctx, params, call) (ToolResponse, error) {
            // Create child session
            session := c.sessions.CreateTaskSession(ctx, id, parentID, title)
            // Run subagent
            result := agent.Run(ctx, call)
            // Aggregate cost to parent
            parentSession.Cost += updatedSession.Cost
            return fantasy.NewTextResponse(result.Response.Content.Text()), nil
        })
}
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `config.Agent` | `internal/config/config.go:336` | Agent configuration struct |
| `config.Agents` | `internal/config/config.go:394` | Map of agent configs |
| `coordinator.buildAgent()` | `internal/agent/coordinator.go` | Creates agents with tools/model |
| `session.CreateTaskSession()` | `internal/session/session.go` | Creates child sessions |
| `SessionAgentOptions.IsSubAgent` | `internal/agent/agent.go:112` | Flag for subagent behavior |

### Agent Config Structure

```go
type Agent struct {
    ID           string
    Name         string
    Description  string
    Disabled     bool
    Model        SelectedModelType  // "large" or "small"
    AllowedTools []string
    AllowedMCP   map[string][]string
    ContextPaths []string
}
```

## Implementation Plan

### Phase 1: Core Infrastructure

#### 1.1 Subagent Definition Parser

Create a new package `internal/subagent/` with:

```go
// internal/subagent/subagent.go

type Subagent struct {
    // From frontmatter
    Name             string            `yaml:"name"`
    Description      string            `yaml:"description"`
    Tools            []string          `yaml:"tools,omitempty"`
    DisallowedTools  []string          `yaml:"disallowedTools,omitempty"`
    Model            string            `yaml:"model,omitempty"`  // sonnet|opus|haiku|inherit
    PermissionMode   string            `yaml:"permissionMode,omitempty"`
    Color            string            `yaml:"color,omitempty"`
    Skills           []string          `yaml:"skills,omitempty"`
    Hooks            *SubagentHooks    `yaml:"hooks,omitempty"`
    
    // Parsed from body
    SystemPrompt     string            `yaml:"-"`
    
    // Metadata
    Source           SubagentSource    `yaml:"-"`  // user|project|cli|plugin
    FilePath         string            `yaml:"-"`
}

type SubagentSource string

const (
    SubagentSourceCLI     SubagentSource = "cli"
    SubagentSourceProject SubagentSource = "project"
    SubagentSourceUser    SubagentSource = "user"
    SubagentSourcePlugin  SubagentSource = "plugin"
)
```

#### 1.2 Subagent Loader

```go
// internal/subagent/loader.go

type Loader struct {
    userDir    string  // ~/.config/crush/agents/
    projectDir string  // .crush/agents/
    cliAgents  string  // --agents flag JSON
}

func (l *Loader) Load() ([]Subagent, error) {
    // 1. Parse CLI JSON agents (highest priority)
    // 2. Load project agents from .crush/agents/*.md
    // 3. Load user agents from ~/.config/crush/agents/*.md
    // 4. Deduplicate by name (higher priority wins)
    // 5. Parse YAML frontmatter + markdown body
    // 6. Validate required fields
}

func ParseSubagentFile(path string) (*Subagent, error) {
    // Uses goldmark or similar to parse YAML frontmatter
    // Body becomes SystemPrompt
}
```

#### 1.3 Extend Config

Add to `internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...
    
    // Loaded from .crush/agents/ and ~/.config/crush/agents/
    Subagents map[string]*subagent.Subagent `json:"-"`
}

// Add CLI flag
type Options struct {
    // ... existing fields ...
    SubagentsJSON string `json:"-"`  // --agents flag
}
```

### Phase 2: Dynamic Tool Generation

#### 2.1 Convert Subagents to Fantasy Tools

Extend coordinator to generate tools from subagent definitions:

```go
// internal/agent/subagent_tool.go

func (c *coordinator) subagentTool(ctx context.Context, sub *subagent.Subagent) (fantasy.AgentTool, error) {
    // 1. Build allowed tools list
    tools := c.resolveSubagentTools(sub)
    
    // 2. Select model
    model := c.resolveSubagentModel(sub)
    
    // 3. Build agent config
    agentCfg := config.Agent{
        ID:           "subagent-" + sub.Name,
        Name:         sub.Name,
        Description:  sub.Description,
        Model:        model,
        AllowedTools: tools,
        AllowedMCP:   c.resolveSubagentMCPs(sub),
    }
    
    // 4. Create agent with custom system prompt
    agent, err := c.buildAgentWithPrompt(ctx, sub.SystemPrompt, agentCfg, true)
    
    // 5. Return as parallel tool (concurrent execution)
    return fantasy.NewParallelAgentTool(
        sub.Name,
        sub.Description,
        c.subagentHandler(agent, sub),
    ), nil
}

func (c *coordinator) resolveSubagentModel(sub *subagent.Subagent) SelectedModelType {
    switch sub.Model {
    case "haiku", "small":
        return SelectedModelTypeSmall
    case "opus", "sonnet", "large", "":
        return SelectedModelTypeLarge
    case "inherit":
        return c.currentModel  // Parent's model
    default:
        return SelectedModelTypeLarge
    }
}
```

#### 2.2 Tool Registration

Modify `coordinator.buildTools()` to include subagent tools:

```go
func (c *coordinator) buildTools(ctx context.Context, cfg config.Agent, isSubAgent bool) ([]fantasy.AgentTool, error) {
    // ... existing tool building ...
    
    // Add subagent tools (only for main agent, not nested)
    if !isSubAgent {
        for _, sub := range c.cfg.Subagents {
            if sub.Disabled {
                continue
            }
            tool, err := c.subagentTool(ctx, sub)
            if err != nil {
                // Log warning, don't fail
                continue
            }
            tools = append(tools, tool)
        }
    }
    
    return tools, nil
}
```

### Phase 3: Model Aliasing

#### 3.1 Model Alias Resolution

Claude Code uses `sonnet`, `opus`, `haiku` aliases. Map these to configured
models:

```go
// internal/subagent/model.go

type ModelAlias string

const (
    ModelAliasSonnet  ModelAlias = "sonnet"
    ModelAliasOpus    ModelAlias = "opus"
    ModelAliasHaiku   ModelAlias = "haiku"
    ModelAliasInherit ModelAlias = "inherit"
)

func (c *Config) ResolveModelAlias(alias string) SelectedModelType {
    switch ModelAlias(alias) {
    case ModelAliasHaiku:
        return SelectedModelTypeSmall
    case ModelAliasOpus, ModelAliasSonnet:
        return SelectedModelTypeLarge
    case ModelAliasInherit:
        // Use parent's model
        return SelectedModelTypeLarge
    default:
        return SelectedModelTypeLarge
    }
}
```

**Future enhancement**: Allow explicit model mapping in config:

```json
{
  "subagent_models": {
    "haiku": {"model": "claude-3-5-haiku-latest", "provider": "anthropic"},
    "sonnet": {"model": "claude-sonnet-4-20250514", "provider": "anthropic"},
    "opus": {"model": "claude-opus-4-20250514", "provider": "anthropic"}
  }
}
```

### Phase 4: Permission Modes

#### 4.1 Permission Mode Handling

```go
// internal/subagent/permissions.go

type PermissionMode string

const (
    PermissionModeDefault           PermissionMode = "default"
    PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
    PermissionModeDontAsk           PermissionMode = "dontAsk"
    PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
    PermissionModePlan              PermissionMode = "plan"  // read-only
)

func (p PermissionMode) ToAgentOptions() SessionAgentOptions {
    switch p {
    case PermissionModeAcceptEdits:
        return SessionAgentOptions{AutoAcceptEdits: true}
    case PermissionModeDontAsk:
        return SessionAgentOptions{DenyPrompts: true}
    case PermissionModeBypassPermissions:
        return SessionAgentOptions{IsYolo: true}
    case PermissionModePlan:
        return SessionAgentOptions{ReadOnly: true}
    default:
        return SessionAgentOptions{}
    }
}
```

### Phase 5: CLI & UI Integration

#### 5.1 CLI Flag

Add `--agents` flag in `internal/cmd/root.go`:

```go
rootCmd.Flags().StringVar(&agentsJSON, "agents", "", 
    "JSON object defining session-only subagents")
```

#### 5.2 Slash Command

Add `/agents` command for managing subagents:

```go
// internal/tui/commands/agents.go

type AgentsCommand struct{}

func (c *AgentsCommand) Execute() {
    // 1. List all subagents (user, project, plugin)
    // 2. Create new subagent (interactive or with Claude)
    // 3. Edit existing subagent
    // 4. Delete subagent
    // 5. Toggle enabled/disabled
}
```

#### 5.3 TUI Indicator

When a subagent is running, show its name and color in the status line:

```go
// internal/tui/components/statusline.go

func (m Model) renderSubagent() string {
    if m.activeSubagent == nil {
        return ""
    }
    return lipgloss.NewStyle().
        Background(m.activeSubagent.Color).
        Render(" " + m.activeSubagent.Name + " ")
}
```

### Phase 6: Hooks Support

#### 6.1 Hook Types

```go
// internal/subagent/hooks.go

type SubagentHooks struct {
    PreToolUse  []HookMatcher `yaml:"PreToolUse,omitempty"`
    PostToolUse []HookMatcher `yaml:"PostToolUse,omitempty"`
    Stop        []Hook        `yaml:"Stop,omitempty"`
}

type HookMatcher struct {
    Matcher string `yaml:"matcher"`  // Regex for tool name
    Hooks   []Hook `yaml:"hooks"`
}

type Hook struct {
    Type    string `yaml:"type"`     // "command"
    Command string `yaml:"command"`  // Shell command to run
}
```

**Scope**: Hooks are a lower priority feature. Initial implementation can skip
hooks and add them in a follow-up.

### Phase 7: Skills Integration

Skills are reusable prompts/workflows that can be injected into subagent system
prompts:

```go
func (c *coordinator) buildSubagentSystemPrompt(sub *subagent.Subagent) string {
    prompt := sub.SystemPrompt
    
    // Inject skills
    for _, skillName := range sub.Skills {
        skill, ok := c.skills.Get(skillName)
        if ok {
            prompt += "\n\n" + skill.Content
        }
    }
    
    return prompt
}
```

**Scope**: Skills integration depends on the existing skills feature. Can be
added after core subagent support is complete.

## File Structure

```
internal/
├── subagent/
│   ├── subagent.go       # Subagent struct and types
│   ├── loader.go         # Load from files and CLI
│   ├── parser.go         # Parse YAML frontmatter + markdown
│   ├── model.go          # Model alias resolution
│   ├── permissions.go    # Permission mode handling
│   └── hooks.go          # Hook types and execution
├── agent/
│   ├── subagent_tool.go  # Generate Fantasy tools from subagents
│   └── coordinator.go    # Modified to register subagent tools
├── config/
│   └── config.go         # Add Subagents field
└── cmd/
    └── root.go           # Add --agents flag
```

## Migration Path

### Compatibility with Claude Code

1. **File format**: Use identical YAML frontmatter schema
2. **Directory structure**: Support both Claude (`~/.claude/agents/`) and Crush
   (`~/.config/crush/agents/`) paths
3. **Tool names**: Map Claude tool names to Crush equivalents:
   - `Read` → `view`
   - `Glob` → `glob`
   - `Grep` → `grep`
   - `Bash` → `bash`
   - `Write` → `write`
   - `Edit` → `edit`

### Symlink Support

Users can symlink their Claude agents directory:

```bash
ln -s ~/.claude/agents ~/.config/crush/agents
```

## Testing Strategy

1. **Unit tests**: Parser, loader, model resolution, permission modes
2. **Integration tests**: Full subagent execution with VCR cassettes
3. **Golden file tests**: Parsed subagent output validation
4. **Compatibility tests**: Verify Claude Code agent files parse correctly

## Rollout Plan

### Phase 1 (MVP)

- [ ] Subagent definition parser (YAML frontmatter + markdown)
- [ ] User-level subagent loading (`~/.config/crush/agents/`)
- [ ] Project-level subagent loading (`.crush/agents/`)
- [ ] Basic tool generation (convert subagent → Fantasy tool)
- [ ] Model alias resolution (sonnet/opus/haiku → large/small)

### Phase 2

- [ ] `--agents` CLI flag for session-only subagents
- [ ] `/agents` slash command for management
- [ ] Tool allow/denylist support
- [ ] Permission mode handling

### Phase 3

- [ ] TUI indicator for active subagent
- [ ] Subagent color support
- [ ] Claude compatibility symlink detection

### Phase 4 (Future)

- [ ] Hooks support (PreToolUse, PostToolUse, Stop)
- [ ] Skills injection
- [ ] Background subagent execution
- [ ] Subagent resumption (continue previous context)
- [ ] Plugin subagents

## Open Questions

1. **Auto-compaction**: Should subagents support auto-compaction like Claude
   Code? Initial implementation can skip this.

2. **Background execution**: Claude Code supports background subagents that
   auto-deny permissions. Priority for initial release?

3. **Subagent nesting**: Should subagents be able to spawn other subagents?
   Claude Code explicitly prevents this. Recommend same restriction.

4. **Cost tracking**: Current implementation aggregates cost to parent. Keep
   this behavior or track separately?

5. **Transcript storage**: Store subagent transcripts separately like Claude
   Code (`subagents/agent-{id}.jsonl`)?

## References

- [Claude Code Subagent Docs](https://docs.anthropic.com/en/docs/claude-code/sub-agents)
- Example agents: `~/.claude/agents/repo-analyzer.md`, `~/.claude/agents/web-search-organic.md`
- Existing agent tool: `internal/agent/agent_tool.go`
