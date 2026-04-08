# CrushCL Architecture Document

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Directory Structure](#2-directory-structure)
3. [Core Components](#3-core-components)
4. [Data Flow](#4-data-flow)
5. [Key Interfaces](#5-key-interfaces)
6. [Compression System](#6-compression-system)
7. [Multi-Agent Coordination](#7-multi-agent-coordination)
8. [Database Schema](#8-database-schema)

---

## 1. Project Overview

CrushCL is a CLI agent framework that combines:
- **Core Layer**: Follows official Crush architecture for stability
- **Kernel Layer**: Claude Code-inspired patterns (4-tier compression, hook pipelines)
- **Magical Layer**: Enhanced capabilities (circuit breakers, swarm coordination, context management)

```
┌─────────────────────────────────────────────────────────────┐
│                        CrushCL                               │
│                                                              │
│  Official Crush (upstream) ←─── 溝通/分析  ───→  各大 AI Agent │
│         │                                                    │
│         │ 同步官方                                            │
│         ▼                                                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              設計/協調/介面標準化                       │   │
│  └─────────────────────────────────────────────────────┘   │
│         │                                                    │
│         │ 傳遞設計意圖                                        │
│         ▼                                                    │
│  CrushCL (Executor) ──→ 實現各 AI Agent 逆向工程的技術結晶   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Directory Structure

```
crushcl/
├── main.go                          # Application entry point
├── AGENTS.md                        # Agent behavior guidelines
│
├── cmd/                             # CLI commands
│   ├── hybrid-brain/                # Hybrid brain subcommand
│   └── claudecode-bridge/          # Claude Code bridge
│
├── internal/
│   ├── agent/                       # Core agent implementation
│   │   ├── agent.go                 # Main SessionAgent implementation
│   │   ├── coordinator.go           # Agent coordinator
│   │   ├── swarm.go                 # Swarm coordination
│   │   ├── swarm_ext.go             # Extended swarm features
│   │   ├── context_manager.go       # Context management
│   │   ├── circuit_breaker.go       # Circuit breaker pattern
│   │   ├── token_estimator.go       # Token counting
│   │   ├── messagebus/              # Message bus system
│   │   ├── aggregator/              # Result aggregation
│   │   ├── guardian/               # Guardian system
│   │   ├── prompt/                 # Prompt templates
│   │   ├── statmachine/             # State machine
│   │   └── tools/                  # Agent tools (MCP, bash, etc.)
│   │
│   ├── kernel/                      # Claude Code patterns
│   │   ├── context/                # Context management
│   │   │   ├── compactor.go        # 4-tier compression
│   │   │   ├── context_manager.go  # Core context manager
│   │   │   ├── session_memory_pool.go
│   │   │   ├── memory_hit_calculator.go
│   │   │   └── sm_composer.go
│   │   ├── loop/                   # State machine
│   │   ├── coordination/           # Coordinator
│   │   ├── hook_pipeline.go        # Hook system
│   │   ├── compression_orchestrator.go
│   │   ├── usage_tracker.go         # Token/cost tracking
│   │   ├── permission/             # Permission system
│   │   ├── registry/               # Tool registry
│   │   └── memory/                 # Memory store
│   │
│   ├── app/                         # Application wiring
│   │   └── app.go                  # Main app, wires services
│   │
│   ├── backend/                     # Backend service
│   ├── client/                      # Client implementation
│   ├── cmd/                         # CLI subcommands
│   ├── commands/                    # Command handlers
│   ├── config/                      # Configuration management
│   ├── db/                          # Database layer (SQLite)
│   │   ├── db.go                   # Generated queries
│   │   ├── models.go               # DB models
│   │   ├── sessions.sql.go         # Session queries
│   │   ├── messages.sql.go        # Message queries
│   │   └── migrations/             # DB migrations
│   │
│   ├── event/                       # Event system
│   ├── message/                    # Message types
│   ├── session/                     # Session management
│   │   └── session.go              # Session service
│   │
│   ├── ui/                          # Terminal UI
│   │   ├── chat/                   # Chat components
│   │   ├── model/                 # TUI model
│   │   └── styles/                # UI styles
│   │
│   ├── server/                      # Server implementation
│   ├── proto/                       # Protocol definitions
│   ├── permission/                  # Permission handling
│   ├── lsp/                         # LSP client integration
│   ├── filetracker/                # File tracking
│   ├── history/                    # History service
│   ├── workspace/                  # Workspace management
│   ├── pubsub/                      # Pub/sub system
│   ├── shell/                       # Shell commands
│   └── ...
│
├── docs/                            # Documentation
│   └── architecture/                # Detailed architecture docs
│
├── collaboration/                  # Collaboration features
│   ├── output/                     # Output handling
│   └── tasks/                     # Task definitions
│
└── memory/                         # Memory storage
    ├── local/                     # Local memory
    └── team/                      # Team memory
```

---

## 3. Core Components

### 3.1 Application Layer (`internal/app/`)

**App** is the main application container that wires all services together.

```go
type App struct {
    Sessions    session.Service       // Session management
    Messages    message.Service      // Message storage
    History     history.Service       // File history
    Permissions permission.Service    // Permission checks
    FileTracker filetracker.Service   // Track file changes
    AgentCoordinator agent.Coordinator // Multi-agent coordination
    LSPManager *lsp.Manager          // Language server protocol
}
```

**Responsibilities:**
- Initialize and wire all services
- Manage application lifecycle (startup/shutdown)
- Coordinate between services
- Handle non-interactive runs

### 3.2 Agent Layer (`internal/agent/`)

#### SessionAgent Interface
The main agent interface that handles AI interactions:

```go
type SessionAgent interface {
    Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
    SetModels(large Model, small Model)
    SetTools(tools []fantasy.AgentTool)
    SetSystemPrompt(systemPrompt string)
    Cancel(sessionID string)
    CancelAll()
    IsSessionBusy(sessionID string) bool
    Summarize(context.Context, string, fantasy.ProviderOptions) error
}
```

#### Coordinator
Manages multiple agents and model configurations:

```go
type coordinator struct {
    cfg           *config.ConfigStore
    sessions      session.Service
    messages      message.Service
    permissions   permission.Service
    currentAgent  SessionAgent
    agents        map[string]SessionAgent
}
```

#### Magical Components (Enhanced Capabilities)
Located in `internal/agent/`:

| Component | File | Purpose |
|-----------|------|---------|
| CircuitBreaker | `circuit_breaker.go` | Retry handling with exponential backoff |
| ContextManager | `context_manager.go` | Enhanced context tracking |
| Swarm | `swarm.go` | Multi-agent task coordination |
| StreamingMonitor | `streaming_monitor.go` | Real-time output monitoring |

#### Kernel Components (Claude Code Patterns)
Located in `internal/kernel/`:

| Component | Purpose |
|-----------|---------|
| ContextCompactor | 4-tier compression system |
| HookPipeline | Pre/post execution hooks |
| CompressionOrchestrator | Layer coordination |
| UsageTracker | Token/cost tracking |
| ToolRegistry | Tool registration/lookup |

### 3.3 Session Management (`internal/session/`)

```go
type Session struct {
    ID               string
    ParentSessionID  string     // For sub-sessions
    Title            string
    MessageCount     int64
    PromptTokens     int64
    CompletionTokens int64
    SummaryMessageID string     // Collapse point
    Cost             float64
    Todos            []Todo
    CreatedAt        int64
    UpdatedAt        int64
}

type Service interface {
    Create(ctx context.Context, title string) (Session, error)
    CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error)
    Get(ctx context.Context, id string) (Session, error)
    Save(ctx context.Context, session Session) (Session, error)
    // ...
}
```

**Session Types:**
- **Main Session**: User conversation
- **Title Session**: Async title generation
- **Task Session**: Sub-agent execution (`messageID$$toolCallID` format)
- **Agent Tool Session**: Nested agent tool calls

### 3.4 Message Layer (`internal/message/`)

Messages are stored with full content for streaming and audit:

```go
type Message struct {
    ID            string
    SessionID     string
    Role          Role          // User, Assistant, Tool
    Parts        []ContentPart  // Text, ToolCall, ToolResult, etc.
    Model         string
    Provider      string
    IsSummaryMessage bool
}
```

---

## 4. Data Flow

### 4.1 Request Flow (Interactive Mode)

```
User Input
    │
    ▼
┌─────────────────┐
│   App.Run()     │  Create/resume session
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Coordinator.Run │  Update models, merge options
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ SessionAgent.Run│  Main processing loop
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌────────┐ ┌────────┐
│ Queue  │ │ Stream │
│ Check  │ │  Loop  │
└────────┘ └────────┘
         │
         ▼
┌─────────────────────────────┐
│   Fantasy Agent.Stream()    │
│                             │
│  ┌─────────────────────┐   │
│  │   PrepareStep      │   │  ← Compression happens here
│  │   - L1/L2/L3/L4    │   │
│  └─────────────────────┘   │
│                             │
│  ┌─────────────────────┐   │
│  │   OnTextDelta       │   │  ← Streaming output
│  │   OnToolCall        │   │
│  │   OnToolResult      │   │
│  │   OnStepFinish      │   │
│  └─────────────────────┘   │
└─────────────────────────────┘
         │
         ▼
┌─────────────────┐
│ Update Session  │  Save usage, cost
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Send to TUI     │  Via pub/sub
└─────────────────┘
```

### 4.2 Compression Flow (4-Tier System)

```
Token Count Check (PrepareStep)
         │
         ▼
┌────────────────────────────────────────────────┐
│  GetCompressionLevel(currentTokens)             │
├────────────────────────────────────────────────┤
│  < 85% budget + tool count > 20  → L1 (<1ms)   │
│  ≥ 85% budget                → L2 (~100ms)    │
│  ≥ 95% budget                → L3 (5-30s)     │
│  ≥ 85% + existing collapses  → L4 (<10ms)      │
└────────────────────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────────┐
│  Execute Compression                            │
├────────────────────────────────────────────────┤
│  L1: Rule-based cleanup (old tool results)      │
│  L2: Threshold-triggered summarization         │
│  L3: Fork agent summarization (small model)     │
│  L4: Session memory projection                  │
└────────────────────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────────┐
│  Safety Check: Verify within limits             │
│  If still over → Emergency truncation           │
└────────────────────────────────────────────────┘
```

### 4.3 Swarm Coordination Flow

```
Task Submission
      │
      ▼
┌──────────────┐
│ SubmitTask() │  Add to pending queue
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Dispatch   │  Match to available agent
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────────────┐
│           Agent Roles                         │
├──────────────────────────────────────────────┤
│  Coordinator ──→ Assigns tasks                │
│  Worker      ──→ Executes tasks              │
│  Reviewer    ──→ Validates results           │
│  Planner     ──→ Creates subtasks            │
└──────────────────────────────────────────────┘
       │
       ▼
┌──────────────┐
│ResultAggregat│  Collect and merge
└──────┬───────┘
       │
       ▼
  Final Result
```

---

## 5. Key Interfaces

### 5.1 Service Interfaces

```go
// Session Service
type SessionService interface {
    Create(ctx context.Context, title string) (Session, error)
    Get(ctx context.Context, id string) (Session, error)
    Save(ctx context.Context, session Session) (Session, error)
    List(ctx context.Context) ([]Session, error)
    Subscribe(ctx context.Context) <-chan Event[Session]
}

// Message Service
type MessageService interface {
    Create(ctx context.Context, sessionID string, params CreateMessageParams) (Message, error)
    List(ctx context.Context, sessionID string) ([]Message, error)
    Update(ctx context.Context, message Message) error
    Delete(ctx context.Context, id string) error
    Subscribe(ctx context.Context) <-chan Event[Message]
}

// Coordinator Interface
type Coordinator interface {
    Run(ctx context.Context, sessionID, prompt string, attachments ...Attachment) (*AgentResult, error)
    Cancel(sessionID string)
    CancelAll()
    IsSessionBusy(sessionID string) bool
    UpdateModels(ctx context.Context) error
    Model() Model
}
```

### 5.2 Provider Interface

```go
// Language Model Provider
type Provider interface {
    LanguageModel(ctx context.Context, modelID string) (LanguageModel, error)
}

// Language Model
type LanguageModel interface {
    Stream(ctx context.Context, call AgentStreamCall) (*AgentResult, error)
    Model() string
    Provider() string
}
```

### 5.3 Hook Interface

```go
type HookPipeline interface {
    RegisterHook(hook *Hook) error
    UnregisterHook(name string) bool
    ExecutePhase(ctx context.Context, phase HookPhase, hookCtx *HookContext) []error
}

type Hook struct {
    Name     string
    Phase    HookPhase       // pre_tool_use, post_tool_use, pre_compact, etc.
    Priority HookPriority    // high, medium, low
    Fn       HookFunc
    Enabled  bool
}

const (
    HookPhasePreToolUse    HookPhase = "pre_tool_use"
    HookPhasePostToolUse   HookPhase = "post_tool_use"
    HookPhasePreCompact    HookPhase = "pre_compact"
    HookPhasePostCompact   HookPhase = "post_compact"
    HookPhaseOnError       HookPhase = "on_error"
)
```

### 5.4 Message Bus Interface

```go
type MessageBus interface {
    Send(ctx context.Context, msg *Message) error
    Broadcast(ctx context.Context, msg *Message) error
    Request(ctx context.Context, msg *Message, timeout time.Duration) (*Message, error)
    Reply(ctx context.Context, original *Message, payload interface{}) error
    Subscribe(sub *Subscription) func()
    GetInbox(agentID string) []*Message
    Shutdown()
}

type Message struct {
    ID        string
    From      string
    To        string
    Type      MessageType
    Payload   interface{}
    Priority  MessagePriority
    Timestamp time.Time
}
```

---

## 6. Compression System

### 6.1 4-Tier Compression Architecture

```
┌─────────────────────────────────────────────────────────────┐
│              CompressionOrchestrator                        │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  HookPipeline                        │   │
│  │  pre_compact → [hooks] → post_compact               │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              ContextCompactor                        │   │
│  │                                                      │   │
│  │  L1Microcompact (<1ms)  ──→ Rule-based cleanup      │   │
│  │  L2AutoCompact (~100ms) ──→ Threshold summary        │   │
│  │  L3FullCompact (5-30s) ──→ Fork agent summary       │   │
│  │  L4SessionMemory (<10ms) ──→ Collapse projection     │   │
│  │                                                      │   │
│  │  ┌─────────────────────────────────────────────┐   │   │
│  │  │         SM Compression Components            │   │   │
│  │  │  SessionMemoryPool │ MemoryHitCalculator    │   │   │
│  │  │              SMComposer                    │   │   │
│  │  └─────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │            EnhancedUsageTracker                      │   │
│  │  Token counting, cost tracking, budget management   │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 Trigger Thresholds

| Tier | Trigger Condition | Latency Target | Mechanism |
|------|-------------------|----------------|-----------|
| **L1** | tool count > 20 | <1ms | Rule-based removal of old tool results |
| **L2** | tokens ≥ 85% budget | ~100ms | Summarize middle messages |
| **L3** | tokens ≥ 95% budget | 5-30s | Fork small model for async summary |
| **L4** | ≥85% + existing collapses | <10ms | Project view from past summaries |

### 6.3 Token Budget Calculation

```go
// Default: 200K tokens
// Safe budget = ContextWindow * (1 - ErrorMargin - SafetyBuffer)
//           = ContextWindow * 0.72

maxTokenBudget := CalculateSafeBudget(contextWindow, 0.20)  // 20% error margin
```

---

## 7. Multi-Agent Coordination

### 7.1 Swarm Architecture

```go
type swarm struct {
    agents     map[string]*AgentInfo
    tasks      map[string]*Task
    messages   chan Message
    taskQueue  chan string
    results    *ResultAggregator
}

type AgentInfo struct {
    ID       string
    Role     AgentRole   // coordinator, worker, reviewer, planner
    Name     string
    Busy     bool
    Tasks    []string
    Capacity int         // Max concurrent tasks
}

type Task struct {
    ID          string
    Description string
    AssignedTo  string
    Status      TaskStatus  // pending, running, completed, failed
    Priority    int
    Result      *AgentResult
}
```

### 7.2 Agent Roles

| Role | Responsibilities |
|------|------------------|
| **Coordinator** | Task assignment, result aggregation |
| **Worker** | Execute assigned tasks |
| **Reviewer** | Validate outputs |
| **Planner** | Decompose complex tasks |

### 7.3 Task Lifecycle

```
TaskPending → TaskRunning → TaskCompleted
                           ↘ TaskFailed
                           ↘ TaskCancelled
```

---

## 8. Database Schema

### 8.1 Core Tables

```sql
-- Sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT,
    title TEXT NOT NULL,
    message_count INTEGER DEFAULT 0,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    summary_message_id TEXT,
    cost REAL DEFAULT 0,
    todos TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Messages table
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    model TEXT,
    provider TEXT,
    is_summary_message BOOLEAN DEFAULT FALSE,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Files table (tracked files)
CREATE TABLE files (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    path TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

### 8.2 Query Layer

Generated via sqlc from `internal/db/sql/`:

```go
type Queries struct {
    db                             DBTX
    createSessionStmt               *sql.Stmt
    getSessionByIDStmt             *sql.Stmt
    updateSessionStmt              *sql.Stmt
    createMessageStmt              *sql.Stmt
    listMessagesBySessionStmt      *sql.Stmt
    // ... 40+ statements
}
```

---

## Appendix: Key Constants

### Compression Levels
```go
const (
    L0None CompressionLevel = iota  // No compression
    L1Microcompact                   // <1ms, rule-based
    L2AutoCompact                   // ~100ms, threshold-based
    L3FullCompact                   // 5-30s, fork summarization
    L4SessionMemory                 // <10ms, collapse projection
)
```

### Token Budgets
```go
DefaultMaxTokenBudget = 200000  // 200K tokens
SafeBudgetMargin      = 0.20    // 20% error margin
SafetyBuffer          = 0.10    // 10% safety buffer
```

### Hook Phases
```go
const (
    HookPhasePreToolUse    HookPhase = "pre_tool_use"
    HookPhasePostToolUse   HookPhase = "post_tool_use"
    HookPhasePreCompact    HookPhase = "pre_compact"
    HookPhasePostCompact   HookPhase = "post_compact"
    HookPhaseOnError       HookPhase = "on_error"
)
```

---

*Document Version: 1.0*
*Last Updated: 2026-04-04*
