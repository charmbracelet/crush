# Crush Plugin System - Product Requirements Document

## Overview

This document describes the design and implementation plan for a plugin system in Crush. The goal is to provide extensible hooks and tool registration capabilities without requiring users to modify the core codebase.

## Background

### Current State

Crush currently has two extension mechanisms:

1. **MCP (Model Context Protocol)** - For external service integration (databases, APIs, etc.)
2. **Skills** - For prompt injection via `SKILL.md` files

### Limitations

| Capability | Current State | Impact |
|------------|---------------|--------|
| Tool interception | Not supported | Cannot audit, sandbox, or customize tool behavior |
| Custom authentication | Hard-coded providers | Adding new providers requires code changes |
| LLM parameter modification | Not supported | Cannot dynamically adjust temperature, headers, etc. |
| Permission customization | Built-in rules only | Cannot implement custom permission logic |
| Command extension | Not supported | Cannot add custom slash commands |
| Agent specialization | Template-only | Cannot define specialized sub-agents |
| Shell environment | Static | Cannot inject dynamic environment variables |

### Comparison with opencode

opencode provides a comprehensive plugin system with:

- `Hooks` interface for lifecycle interception
- Dynamic tool registration
- Authentication provider extension
- LLM request/response hooks
- Permission decision hooks
- Shell environment injection
- Local tool files (`.opencode/tool/*.ts`)
- Agent definition files (`.opencode/agent/*.md`)
- Command extension (`.opencode/command/*.md`)

## Goals

### Primary Goals

1. **Hooks System** - Provide lifecycle hooks for key extension points
2. **Tool Registration** - Allow users to define custom tools in `~/.config/crush/tools/`
3. **Backward Compatibility** - Existing MCP and Skills mechanisms remain unchanged

### Non-Goals

- npm package plugin loading (future phase)
- WebAssembly runtime (future phase)
- Remote plugin distribution (future phase)

## Implementation Status (Updated)

### Completed

- [x] Hook interfaces and plugin manager are implemented in `internal/plugin/plugin.go` and `internal/plugin/manager.go`.
- [x] Shell environment hook is integrated via `SetRuntimeEnvHook()` in `internal/shell/shell.go`.
- [x] Permission hook is integrated via `TriggerPermissionAsk()` in `internal/permission/permission.go`.
- [x] Tool before/after hooks are integrated globally via `WrapAgentTool()` in `internal/plugin/agent_tool.go` and tool wiring in `internal/agent/coordinator.go`.
- [x] Chat before/after hooks are integrated in `internal/agent/agent.go` via `TriggerChatBeforeRequest()` and `TriggerChatAfterResponse()`.
- [x] Message-created hook is integrated in app event wiring via `setupMessageSubscriber()` in `internal/app/app.go` and `TriggerMessageCreated()`.
- [x] Plugin initialization is wired into application startup via `plugin.Init()` in `internal/app/app.go`.
- [x] Local tool discovery is implemented for both global and project scopes:
  - `~/.config/crush/tools/*.json`
  - `<project>/.crush/tools/*.json`
- [x] Command extension is implemented via `internal/commands/commands.go` and UI integration in `internal/ui/model/ui.go`.

### Completed Tests

- [x] Unit tests for plugin manager and hook dispatch in `internal/plugin/manager_test.go`.
- [x] Unit tests for shell env integration in `internal/shell/shell_test.go`.
- [x] Unit tests for permission integration in `internal/permission/permission_test.go`.
- [x] Integration tests for chat hook execution path in `internal/agent/plugin_chat_hooks_test.go`.
- [x] Integration tests for local tool discovery and execution precedence in `internal/plugin/manager_test.go`.

### Pending / Future

- [ ] Agent definition extension via local files remains future work.

## Technical Design

### Phase 1: Hooks System

#### 1.1 Core Interfaces

```go
// internal/plugin/plugin.go
package plugin

import (
    "context"
    
    "charm.land/fantasy"
    "github.com/charmbracelet/crush/internal/config"
    "github.com/charmbracelet/crush/internal/message"
    "github.com/charmbracelet/crush/internal/permission"
    "github.com/charmbracelet/crush/internal/session"
)

// PluginInput provides context for plugin initialization.
type PluginInput struct {
    Config     *config.ConfigStore
    Sessions   session.Service
    Messages   message.Service
    WorkingDir string
}

// Hooks defines all available extension points.
type Hooks struct {
    // Tool execution lifecycle
    ToolBeforeExecute func(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error)
    ToolAfterExecute  func(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error)
    
    // LLM request lifecycle
    ChatBeforeRequest func(ctx context.Context, input ChatBeforeRequestInput) error
    ChatAfterResponse func(ctx context.Context, input ChatAfterResponseInput) error
    
    // Permission handling
    PermissionAsk func(input PermissionAskInput) PermissionAskOutput
    
    // Shell environment injection
    ShellEnv func(ctx context.Context, input ShellEnvInput) map[string]string
    
    // Message lifecycle
    MessageCreated func(ctx context.Context, msg message.Message) error
    
    // Custom tool definitions
    Tools map[string]ToolDefinition
}

// ToolDefinition describes a custom tool.
type ToolDefinition struct {
    Name        string
    Description string
    Parameters  any // JSON Schema
    Execute     func(ctx context.Context, args map[string]any, ctx2 ToolContext) (string, error)
}

// ToolContext provides context for tool execution.
type ToolContext struct {
    SessionID string
    MessageID string
    Agent     string
    Directory string
    Worktree  string
}

// Input/Output types for hooks
type ToolBeforeExecuteInput struct {
    Tool      string
    SessionID string
    CallID    string
    Args      map[string]any
}

type ToolBeforeExecuteOutput struct {
    Args      map[string]any // Modified args
    Skip      bool           // If true, skip execution
    PreResult string         // If Skip is true, use this as result
}

type ToolAfterExecuteInput struct {
    Tool      string
    SessionID string
    CallID    string
    Args      map[string]any
    Result    string
    Metadata  map[string]any
}

type ToolAfterExecuteOutput struct {
    Result   string // Modified result
    Metadata map[string]any
}

type ChatBeforeRequestInput struct {
    SessionID string
    Agent     string
    Model     ModelInfo
    Provider  ProviderContext
    Message   message.Message
}

type ChatAfterResponseInput struct {
    SessionID string
    Agent     string
    Model     ModelInfo
    Result    *fantasy.AgentResult
    Error     error
}

type PermissionAskInput struct {
    Permission permission.PermissionRequest
}

type PermissionAskOutput struct {
    Action permission.PermissionAction
}

type ShellEnvInput struct {
    CWD       string
    SessionID string
    CallID    string
}

type ModelInfo struct {
    ProviderID string
    ModelID    string
}

type ProviderContext struct {
    Source  string // "env", "config", "custom", "api"
    Options map[string]any
}
```

#### 1.2 Plugin Interface

```go
// Plugin is the interface that all plugins must implement.
type Plugin interface {
    // Name returns the plugin identifier.
    Name() string
    
    // Init initializes the plugin and returns hooks.
    Init(ctx context.Context, input PluginInput) (Hooks, error)
}
```

#### 1.3 Plugin Manager

```go
// internal/plugin/manager.go
package plugin

import (
    "context"
    "log/slog"
    "sync"
)

var (
    plugins []Plugin
    hooks   Hooks
    mu      sync.RWMutex
)

// Register adds a plugin to the registry.
func Register(p Plugin) {
    mu.Lock()
    defer mu.Unlock()
    plugins = append(plugins, p)
}

// Init initializes all registered plugins.
func Init(ctx context.Context, input PluginInput) error {
    mu.Lock()
    defer mu.Unlock()
    
    hooks = Hooks{
        Tools: make(map[string]ToolDefinition),
    }
    
    for _, p := range plugins {
        slog.Info("Initializing plugin", "name", p.Name())
        h, err := p.Init(ctx, input)
        if err != nil {
            slog.Error("Failed to initialize plugin", "name", p.Name(), "error", err)
            continue
        }
        
        // Merge hooks
        if h.ToolBeforeExecute != nil {
            hooks.ToolBeforeExecute = h.ToolBeforeExecute
        }
        if h.ToolAfterExecute != nil {
            hooks.ToolAfterExecute = h.ToolAfterExecute
        }
        if h.ChatBeforeRequest != nil {
            hooks.ChatBeforeRequest = h.ChatBeforeRequest
        }
        if h.ChatAfterResponse != nil {
            hooks.ChatAfterResponse = h.ChatAfterResponse
        }
        if h.PermissionAsk != nil {
            hooks.PermissionAsk = h.PermissionAsk
        }
        if h.ShellEnv != nil {
            hooks.ShellEnv = h.ShellEnv
        }
        if h.MessageCreated != nil {
            hooks.MessageCreated = h.MessageCreated
        }
        for name, tool := range h.Tools {
            hooks.Tools[name] = tool
        }
    }
    
    return nil
}

// GetHooks returns the merged hooks.
func GetHooks() Hooks {
    mu.RLock()
    defer mu.RUnlock()
    return hooks
}

// TriggerToolBefore executes all tool before hooks.
func TriggerToolBefore(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error) {
    mu.RLock()
    hook := hooks.ToolBeforeExecute
    mu.RUnlock()
    
    if hook == nil {
        return nil, nil
    }
    return hook(ctx, input)
}

// TriggerToolAfter executes all tool after hooks.
func TriggerToolAfter(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error) {
    mu.RLock()
    hook := hooks.ToolAfterExecute
    mu.RUnlock()
    
    if hook == nil {
        return nil, nil
    }
    return hook(ctx, input)
}

// GetCustomTools returns all registered custom tools.
func GetCustomTools() map[string]ToolDefinition {
    mu.RLock()
    defer mu.RUnlock()
    return hooks.Tools
}
```

#### 1.4 Hook Integration Points

**Tool Execution** (`internal/agent/tools/bash.go`):

```go
func (t *BashTool) Execute(ctx context.Context, params BashParams) (string, error) {
    // Before hook
    beforeOut, err := plugin.TriggerToolBefore(ctx, plugin.ToolBeforeExecuteInput{
        Tool:      BashToolName,
        SessionID: ctx.Value(tools.SessionIDContextKey).(string),
        CallID:    ctx.Value(tools.CallIDContextKey).(string),
        Args:      paramsToMap(params),
    })
    if err != nil {
        return "", err
    }
    if beforeOut != nil && beforeOut.Skip {
        return beforeOut.PreResult, nil
    }
    if beforeOut != nil {
        params = mapToParams(beforeOut.Args)
    }
    
    // Execute tool
    result, err := t.executeInternal(ctx, params)
    
    // After hook
    afterOut, err := plugin.TriggerToolAfter(ctx, plugin.ToolAfterExecuteInput{
        Tool:      BashToolName,
        SessionID: ctx.Value(tools.SessionIDContextKey).(string),
        CallID:    ctx.Value(tools.CallIDContextKey).(string),
        Args:      paramsToMap(params),
        Result:    result,
    })
    if afterOut != nil {
        result = afterOut.Result
    }
    
    return result, err
}
```

**Shell Environment** (`internal/shell/shell.go`):

```go
func (s *Shell) buildEnv(ctx context.Context, opts *Options) []string {
    env := os.Environ()
    
    // Plugin hook for environment injection
    hook := plugin.GetHooks().ShellEnv
    if hook != nil {
        pluginEnv := hook(ctx, plugin.ShellEnvInput{
            CWD:       opts.WorkingDir,
            SessionID: opts.SessionID,
            CallID:    opts.CallID,
        })
        for k, v := range pluginEnv {
            env = append(env, fmt.Sprintf("%s=%s", k, v))
        }
    }
    
    return env
}
```

**Permission Decision** (`internal/permission/service.go`):

```go
func (s *Service) Ask(req PermissionRequest) PermissionAction {
    // Check plugin hook first
    hook := plugin.GetHooks().PermissionAsk
    if hook != nil {
        out := hook(PermissionAskInput{Permission: req})
        if out.Action != PermissionActionAsk {
            return out.Action
        }
    }
    
    // Default behavior
    return s.askInternal(req)
}
```

### Phase 2: Local Tool Registration

#### 2.1 Tool Discovery

```go
// internal/plugin/tools.go
package plugin

import (
    "encoding/json"
    "os"
    "path/filepath"
    
    "github.com/charmbracelet/crush/internal/config"
)

// DiscoverTools scans tool directories for custom tool definitions.
func DiscoverTools(cfg *config.ConfigStore) ([]ToolDefinition, error) {
    var tools []ToolDefinition
    
    dirs := []string{
        filepath.Join(config.ConfigDir(), "tools"),
        filepath.Join(cfg.WorkingDir(), ".crush", "tools"),
    }
    
    for _, dir := range dirs {
        files, err := filepath.Glob(filepath.Join(dir, "*.json"))
        if err != nil {
            continue
        }
        
        for _, file := range files {
            tool, err := loadToolDefinition(file)
            if err != nil {
                slog.Warn("Failed to load tool definition", "path", file, "error", err)
                continue
            }
            tools = append(tools, tool)
        }
    }
    
    return tools, nil
}

func loadToolDefinition(path string) (ToolDefinition, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return ToolDefinition{}, err
    }
    
    var def struct {
        Name        string          `json:"name"`
        Description string          `json:"description"`
        Parameters  json.RawMessage `json:"parameters"`
        Execute     string          `json:"execute"` // Command to execute
    }
    
    if err := json.Unmarshal(data, &def); err != nil {
        return ToolDefinition{}, err
    }
    
    return ToolDefinition{
        Name:        def.Name,
        Description: def.Description,
        Parameters:  def.Parameters,
        Execute:     createCommandExecutor(def.Execute),
    }, nil
}
```

#### 2.2 Tool Definition Format

**File**: `~/.config/crush/tools/github-issue.json`

```json
{
  "name": "github_issue",
  "description": "Create or update GitHub issues",
  "parameters": {
    "type": "object",
    "properties": {
      "title": { "type": "string", "description": "Issue title" },
      "body": { "type": "string", "description": "Issue body" },
      "labels": { "type": "array", "items": { "type": "string" } }
    },
    "required": ["title"]
  },
  "execute": "gh issue create --title \"$TITLE\" --body \"$BODY\""
}
```

### Phase 3: Command Extension

#### 3.1 Command Discovery

```go
// internal/plugin/commands.go
package plugin

// CommandDefinition describes a custom command.
type CommandDefinition struct {
    Name        string
    Description string
    Template    string
}

// DiscoverCommands scans for custom command definitions.
func DiscoverCommands(cfg *config.ConfigStore) ([]CommandDefinition, error) {
    // Similar to tool discovery
    // Looks for ~/.config/crush/commands/*.md and .crush/commands/*.md
}
```

#### 3.2 Command Definition Format

**File**: `~/.config/crush/commands/commit.md`

```markdown
---
name: commit
description: Generate a commit message following conventional commits
---

Analyze the staged changes and generate a commit message following the conventional commits specification. The message should:

1. Start with a type: feat, fix, docs, style, refactor, test, chore
2. Include a scope in parentheses if applicable
3. Have a concise subject line (under 72 characters)
4. Optionally include a body explaining the "why"

Staged changes:
` + "```" + `
{{.StagedDiff}}
` + "```" + `
```

## Directory Structure

```
~/.config/crush/
├── tools/
│   ├── github-issue.json
│   ├── jira-search.json
│   └── ...
├── commands/
│   ├── commit.md
│   ├── review.md
│   └── ...
└── plugins/
    ├── audit/
    │   ├── SKILL.md
    │   └── plugin.go
    └── sandbox/
        ├── SKILL.md
        └── plugin.go

<project>/.crush/
├── tools/
│   └── project-specific.json
└── commands/
    └── project-specific.md
```

## Implementation Plan

### Phase 1: Core Hooks System (Week 1)

| Task | Priority | Effort |
|------|----------|--------|
| Define plugin interfaces | P0 | 4h |
| Implement plugin manager | P0 | 4h |
| Add hooks to bash tool | P0 | 2h |
| Add hooks to edit/write tools | P0 | 2h |
| Add shell env hook | P1 | 2h |
| Add permission hook | P1 | 2h |
| Add chat before/after hooks | P0 | 2h |
| Write unit/integration tests | P0 | 4h |

### Phase 2: Local Tools (Week 2)

| Task | Priority | Effort |
|------|----------|--------|
| Implement tool discovery | P0 | 4h |
| Create command executor | P0 | 4h |
| Integrate with tool registry | P0 | 2h |
| Add documentation | P1 | 2h |
| Write unit/integration tests (including global/project precedence) | P0 | 4h |

### Phase 3: Commands Extension (Week 3)

| Task | Priority | Effort |
|------|----------|--------|
| Implement command discovery | P1 | 4h |
| Create command template engine | P1 | 4h |
| Integrate with command dialog | P1 | 4h |
| Add documentation | P1 | 2h |
| Write unit tests | P1 | 4h |

## Example Plugins

### 1. Audit Plugin

Logs all tool executions to a file for compliance (example-only; production code should avoid recording sensitive data).

```go
package main

import (
    "context"
    "encoding/json"
    "log/slog"
    "os"
    "time"
    
    "github.com/charmbracelet/crush/plugin"
)

type AuditPlugin struct{}

func (p *AuditPlugin) Name() string { return "audit" }

func (p *AuditPlugin) Init(ctx context.Context, input plugin.PluginInput) (plugin.Hooks, error) {
    file, err := os.OpenFile("audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return plugin.Hooks{}, err
    }
    
    return plugin.Hooks{
        ToolAfterExecute: func(ctx context.Context, input plugin.ToolAfterExecuteInput) (*plugin.ToolAfterExecuteOutput, error) {
            entry := map[string]any{
                "timestamp": time.Now().Format(time.RFC3339),
                "tool":      input.Tool,
                "session":   input.SessionID,
                "args":      input.Args,
                "result":    input.Result,
            }
            data, _ := json.Marshal(entry)
            file.WriteString(string(data) + "\n")
            return nil, nil
        },
    }, nil
}

func init() {
    plugin.Register(&AuditPlugin{})
}
```

### 2. Chat Metrics Plugin

Captures per-request metadata before and after model execution.

```go
package main

import (
    "context"
    "log/slog"

    "github.com/charmbracelet/crush/internal/plugin"
)

type ChatMetricsPlugin struct{}

func (p *ChatMetricsPlugin) Name() string { return "chat-metrics" }

func (p *ChatMetricsPlugin) Init(ctx context.Context, input plugin.PluginInput) (plugin.Hooks, error) {
    return plugin.Hooks{
        ChatBeforeRequest: func(ctx context.Context, in plugin.ChatBeforeRequestInput) error {
            slog.Info("Chat start", "session", in.SessionID, "provider", in.Model.ProviderID, "model", in.Model.ModelID)
            return nil
        },
        ChatAfterResponse: func(ctx context.Context, in plugin.ChatAfterResponseInput) error {
            slog.Info("Chat end", "session", in.SessionID, "err", in.Error != nil)
            return nil
        },
    }, nil
}

func init() {
    plugin.Register(&ChatMetricsPlugin{})
}
```

### 3. Local Tool Example

A local tool file can be loaded from `~/.config/crush/tools/` or `<project>/.crush/tools/`.

```json
{
  "name": "echo_arg",
  "description": "Echo a provided argument",
  "parameters": {
    "type": "object",
    "properties": {
      "value": { "type": "string" }
    },
    "required": ["value"]
  },
  "execute": "printf \"%s\" \"$VALUE\""
}
```

Arguments are exported as uppercase environment variables before execution (`value` → `VALUE`).

### 4. Custom Provider Plugin

Adds support for a custom LLM provider.

```go
package main

import (
    "context"
    
    "github.com/charmbracelet/crush/plugin"
)

type CustomProviderPlugin struct{}

func (p *CustomProviderPlugin) Name() string { return "custom-provider" }

func (p *CustomProviderPlugin) Init(ctx context.Context, input plugin.PluginInput) (plugin.Hooks, error) {
    return plugin.Hooks{
        ChatBeforeRequest: func(ctx context.Context, input plugin.ChatBeforeRequestInput) error {
            // Add custom headers for specific provider
            if input.Provider.Source == "custom" {
                // Modify provider options
            }
            return nil
        },
        ShellEnv: func(ctx context.Context, input plugin.ShellEnvInput) map[string]string {
            return map[string]string{
                "CUSTOM_API_URL": "https://api.custom-llm.com",
            }
        },
    }, nil
}

func init() {
    plugin.Register(&CustomProviderPlugin{})
}
```

## Migration Path

### For Users

1. Create `~/.config/crush/plugins/` directory
2. Add plugin files (Go source or JSON tools)
3. Restart Crush

### For Contributors

1. Import `github.com/charmbracelet/crush/plugin`
2. Implement `Plugin` interface
3. Register with `plugin.Register()`

## Open Questions

1. **Plugin isolation**: Should plugins run in separate processes for safety?
   - Recommendation: Phase 1 uses in-process, Phase 4 adds process isolation

2. **Plugin versioning**: How to handle breaking changes in hook interfaces?
   - Recommendation: Semantic versioning with interface stability guarantees

3. **Hot reloading**: Should plugins be reloadable without restart?
   - Recommendation: Not in Phase 1, consider for future

4. **Plugin dependencies**: How to manage dependencies between plugins?
   - Recommendation: No dependencies in Phase 1, explicit ordering later

## Success Metrics

- Time to add a new tool: < 5 minutes (vs. current: requires code change)
- Plugin load time: < 100ms
- Zero breaking changes to existing MCP/Skills usage
- At least 3 community-contributed plugins within 3 months of release

## References

- [opencode Plugin System](https://github.com/anomaly/opencode/tree/main/packages/plugin)
- [Agent Skills Specification](https://agentskills.io)
- [MCP Specification](https://modelcontextprotocol.io)