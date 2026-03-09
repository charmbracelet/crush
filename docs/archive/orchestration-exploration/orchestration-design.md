# Emergent Agent Orchestration Design

## Executive Summary

This document synthesizes patterns from OpenAI Agents SDK, Letta/MemGPT, AutoGen, and CrewAI to design an emergent orchestration system for Crush where AI decides the logic structure—not visual node editors.

**Core Philosophy**: Pre-configured orchestration primitives + AI-decided logic = Emergent behavior

---

## Pattern Analysis Summary

### 1. OpenAI Agents SDK (Minimalist)

**Key Insight**: 3 primitives only - Agents, Handoffs, Guardrails

```
Agent
├── instructions (system prompt)
├── tools[] (function tools)
├── handoffs[] (other agents as delegation targets)
├── model (LLM config)
└── output_type (structured output)

Handoff = Agent as Tool
- Handoffs transfer conversation control (agent takes over)
- as_tool() keeps calling agent in control
```

**Patterns**:
- `handoff(agent)` → Creates `transfer_to_{agent_name}` tool
- `agent.as_tool()` → Agent becomes callable tool, returns result to caller
- `input_filter` → Filter/modifies conversation history before handoff

**Best Idea**: Agents as first-class tools. Handoff = delegation, as_tool = subprocess.

---

### 2. Letta/MemGPT (Memory-Centric)

**Key Insight**: Hierarchical memory + "sleep-time compute" for learning

```
Memory Architecture:
├── Core Memory (always in context)
├── Archival Memory (searchable, infinite)
├── Context Window (rolling, managed)
└── Self-editing blocks (persona, human, tasks)

Multi-Agent Patterns:
├── SupervisorMultiAgent (commented out - WIP)
├── DynamicMultiAgent (manager chooses next speaker)
├── RoundRobinMultiAgent (fixed rotation)
└── SleeptimeMultiAgent (background processing)
```

**SleeptimeMultiAgent Pattern**:
1. Main agent handles user interaction
2. Every N turns, spawn background agents
3. Background agents process conversation history
4. Results stored in memory for future context

**Best Ideas**:
- Sleep-time compute (background learning)
- Self-editing memory blocks
- Agent personas are stored as editable blocks

---

### 3. AutoGen (Conversational)

**Key Insight**: Group chat patterns with different selection strategies

```
Team Patterns:
├── RoundRobinGroupChat (fixed rotation)
├── SelectorGroupChat (LLM chooses next speaker)
├── Swarm (handoff-based routing)
├── DiGraphGroupChat (DAG-based flow)
└── MagenticOne (ledger-based orchestration)

MagenticOne Pattern:
├── Task Ledger (facts + plan)
├── Progress Ledger (tracks completion, stalls)
├── Outer Loop (re-planning when stuck)
└── Inner Loop (agent selection + execution)
```

**MagenticOne Ledgers**:
```json
{
  "is_request_satisfied": {"answer": false, "reason": "..."},
  "is_progress_being_made": {"answer": true, "reason": "..."},
  "is_in_loop": {"answer": false, "reason": "..."},
  "instruction_or_question": {"answer": "...", "reason": "..."},
  "next_speaker": {"answer": "AgentName", "reason": "..."}
}
```

**Best Ideas**:
- Ledger-based orchestration (facts, plan, progress)
- Stall detection + re-planning
- Model-driven speaker selection

---

### 4. CrewAI (Role-Based)

**Key Insight**: Agents as roles in crews with process types

```
Crew
├── agents[] (role-based)
├── tasks[] (sequential assignments)
└── process (sequential | hierarchical | parallel)

Agent Role:
├── role (job title)
├── goal (objective)
├── backstory (context)
└── tools[] (capabilities)
```

**Best Idea**: Role-based agent definition with clear responsibilities

---

## Design: Emergent Orchestration for Crush

### Core Principles

1. **No Visual Node Editors** - AI decides logic at runtime
2. **Pre-configured Primitives** - Well-defined building blocks
3. **Cross-CLI Support** - Works with codex, claude, gemini, crush
4. **Git-Backed Isolation** - Each agent works in a git worktree
5. **AI-Driven Routing** - Model decides who speaks next
6. **Commits as Messages** - Git history is the audit trail

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Orchestrator (Git-Backed)                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  ┌────────────┐  │
│  │ Task Ledger │  │ Progress    │  │ Agent Registry  │  │ Worktree   │  │
│  │ (facts,plan)│  │ Ledger      │  │ (name, role,    │  │ Manager    │  │
│  │ [.orchestra]│  │ [.orchestra]│  │  tools, model)  │  │ (git)      │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  └────────────┘  │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────┐          │
│  │              Router (AI-Driven)                            │          │
│  │  - Selects next agent based on ledger + history           │          │
│  │  - Detects stalls → triggers re-planning                  │          │
│  │  - Detects completion → synthesizes result                │          │
│  │  - Coordinates via git commits/merges                     │          │
│  └───────────────────────────────────────────────────────────┘          │
│                                                                          │
│  Git Operations: commits = messages, branches = tickets, worktrees      │
└─────────────────────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
    ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
    │   worktree   │ │   worktree   │ │   worktree   │
    │   /planner   │ │   /coder     │ │   /reviewer  │
    │              │ │              │ │              │
    │ branch:      │ │ branch:      │ │ branch:      │
    │ task/T-001/  │ │ task/T-001/  │ │ task/T-001/  │
    │ planner      │ │ coder        │ │ reviewer     │
    │              │ │              │ │              │
    │ CLI: crush   │ │ CLI: crush   │ │ CLI: codex   │
    │ Model: glm-4 │ │ Model: claude│ │ Model: o3    │
    └──────────────┘ └──────────────┘ └──────────────┘
                           │
                  merge to task/T-001
```

**Key Insight**: Git is the backbone:
- **Worktrees** = Isolated agent workspaces (no file conflicts)
- **Commits** = Agent messages/actions (full audit trail)
- **Branches** = Tickets/tasks (natural task tracking)
- **Merges** = Handoffs and coordination (conflict resolution built-in)

### Core Types (Go)

```go
// Agent definition
type Agent struct {
    Name            string            `json:"name"`
    Role            string            `json:"role"`            // e.g., "code reviewer"
    Description     string            `json:"description"`     // Used for routing
    Instructions    string            `json:"instructions"`    // System prompt
    Tools           []string          `json:"tools"`           // Available tools
    CLI             string            `json:"cli"`             // "crush", "codex", "claude"
    Model           string            `json:"model"`           // Model override
    Handoffs        []string          `json:"handoffs"`        // Agents it can delegate to
    IsParallel      bool              `json:"is_parallel"`     // Can run in parallel
}

// Task ledger (persistent state)
type TaskLedger struct {
    ID              string            `json:"id"`
    OriginalTask    string            `json:"original_task"`
    Facts           []string          `json:"facts"`           // Discovered facts
    Plan            []PlanStep        `json:"plan"`            // Current plan
    CompletedSteps  []string          `json:"completed_steps"` // Done items
    CreatedAt       time.Time         `json:"created_at"`
    UpdatedAt       time.Time         `json:"updated_at"`
}

type PlanStep struct {
    ID              string            `json:"id"`
    Description     string            `json:"description"`
    AssignedAgent   string            `json:"assigned_agent"`
    Status          string            `json:"status"` // pending, in_progress, completed, failed
    Dependencies    []string          `json:"dependencies"`
}

// Progress ledger (runtime state)
type ProgressLedger struct {
    CurrentAgent    string            `json:"current_agent"`
    TurnCount       int               `json:"turn_count"`
    StallCount      int               `json:"stall_count"`
    MaxStalls       int               `json:"max_stalls"`
    Messages        []Message         `json:"messages"`
    LastResult      string            `json:"last_result"`
    IsComplete      bool              `json:"is_complete"`
    CompletionReason string           `json:"completion_reason,omitempty"`
}

// Router decision
type RouterDecision struct {
    NextAgent       string            `json:"next_agent"`
    Instruction     string            `json:"instruction"`
    IsComplete      bool              `json:"is_complete"`
    NeedsReplanning bool              `json:"needs_replanning"`
    Reason          string            `json:"reason"`
}

// Team configuration
type Team struct {
    Name            string            `json:"name"`
    Description     string            `json:"description"`
    Agents          []Agent           `json:"agents"`
    Orchestrator    OrchestratorType  `json:"orchestrator"`
    MaxTurns        int               `json:"max_turns"`
    MaxStalls       int               `json:"max_stalls"`
}

type OrchestratorType string

const (
    OrchestratorSwarm       OrchestratorType = "swarm"       // Handoff-based
    OrchestratorSelector    OrchestratorType = "selector"    // AI selects next
    OrchestratorRoundRobin  OrchestratorType = "round_robin" // Fixed rotation
    OrchestratorMagentic    OrchestratorType = "magentic"    // Ledger-based
    OrchestratorDynamic     OrchestratorType = "dynamic"     // AI decides pattern
)
```

### Key Components

#### 1. Agent Registry
Manages available agents and their capabilities.

```go
type AgentRegistry struct {
    agents map[string]*AgentInstance
}

type AgentInstance struct {
    Definition Agent
    Process    *exec.Cmd
    Stdin      io.WriteCloser
    Stdout     io.Reader
    Status     AgentStatus
}
```

#### 2. Router (AI-Driven)
Decides which agent should act next.

```go
type Router interface {
    SelectNextAgent(ctx context.Context, ledger *TaskLedger, progress *ProgressLedger) (*RouterDecision, error)
}

// AI-powered router
type AIRouter struct {
    model   llm.Provider
    prompt  string
}

func (r *AIRouter) SelectNextAgent(ctx context.Context, ledger *TaskLedger, progress *ProgressLedger) (*RouterDecision, error) {
    prompt := r.buildPrompt(ledger, progress)
    response, err := r.model.Complete(ctx, prompt)
    // Parse JSON response into RouterDecision
    return parseDecision(response)
}
```

#### 3. Subprocess Agent Executor
Spawns and manages agent processes.

```go
type SubprocessExecutor struct {
    registry *AgentRegistry
}

func (e *SubprocessExecutor) Execute(ctx context.Context, agent *Agent, input string) (*AgentResult, error) {
    // Build CLI command based on agent.CLI
    cmd := e.buildCommand(agent, input)

    // Spawn process
    instance, err := e.registry.Spawn(agent.Name, cmd)
    if err != nil {
        return nil, err
    }

    // Stream output
    output, err := e.streamOutput(ctx, instance)
    if err != nil {
        return nil, err
    }

    return &AgentResult{
        AgentName: agent.Name,
        Output:    output,
        Success:   true,
    }, nil
}

func (e *SubprocessExecutor) ExecuteParallel(ctx context.Context, agents []*Agent, input string) ([]*AgentResult, error) {
    var wg sync.WaitGroup
    results := make([]*AgentResult, len(agents))
    errors := make([]error, len(agents))

    for i, agent := range agents {
        if !agent.IsParallel {
            continue
        }
        wg.Add(1)
        go func(idx int, a *Agent) {
            defer wg.Done()
            results[idx], errors[idx] = e.Execute(ctx, a, input)
        }(i, agent)
    }

    wg.Wait()
    return results, nil
}
```

#### 4. Ledger Manager
Manages task and progress state.

```go
type LedgerManager struct {
    taskLedger     *TaskLedger
    progressLedger *ProgressLedger
}

func (m *LedgerManager) UpdateFacts(facts []string) {
    m.taskLedger.Facts = facts
    m.taskLedger.UpdatedAt = time.Now()
}

func (m *LedgerManager) RecordResult(agent string, result string) {
    m.progressLedger.LastResult = result
    m.progressLedger.Messages = append(m.progressLedger.Messages, Message{
        Agent:    agent,
        Content:  result,
        Time:     time.Now(),
    })
}

func (m *LedgerManager) CheckStall() bool {
    // Detect if no progress is being made
    // Compare last N messages for similarity
    return false
}
```

### Orchestrator Flow

```
1. Initialize
   ├── Create TaskLedger from user request
   ├── Generate initial plan (AI)
   └── Spawn all agents as subprocesses

2. Loop (until complete or max turns)
   ├── Router.SelectNextAgent()
   │   ├── Build prompt with ledger + history
   │   ├── Call LLM to decide
   │   └── Parse RouterDecision
   │
   ├── If needs_replanning:
   │   ├── Update TaskLedger.Facts
   │   ├── Regenerate plan
   │   └── Reset stall counter
   │
   ├── If is_complete:
   │   ├── Synthesize final result
   │   └── Return to user
   │
   ├── Execute agent (subprocess)
   │   ├── Send instruction via stdin
   │   ├── Stream output
   │   └── Record result in ProgressLedger
   │
   └── Check stall condition
       └── If stalled > max_stalls: trigger re-planning

3. Cleanup
   └── Terminate all subprocesses
```

### Tools for Agents

Each agent gets orchestration-aware tools:

```go
// Handoff tool - delegate to another agent
var HandoffTool = tool.Tool{
    Name:        "handoff_to",
    Description: "Delegate the current task to another agent",
    Parameters: map[string]any{
        "agent_name": "string",
        "task": "string",
        "context": "string (optional)",
    },
    Execute: func(args map[string]any) (string, error) {
        // Signal to orchestrator to switch agents
        // Pass accumulated context
    },
}

// Broadcast tool - send message to all agents
var BroadcastTool = tool.Tool{
    Name:        "broadcast",
    Description: "Send a message visible to all agents in the team",
    Parameters: map[string]any{
        "message": "string",
    },
}

// Update facts tool - add discovered information
var UpdateFactsTool = tool.Tool{
    Name:        "add_fact",
    Description: "Add a discovered fact to the task ledger",
    Parameters: map[string]any{
        "fact": "string",
    },
}

// Request parallel execution
var ParallelTool = tool.Tool{
    Name:        "run_parallel",
    Description: "Execute multiple agents in parallel",
    Parameters: map[string]any{
        "agents": "[]string (agent names)",
        "task": "string (same task for all)",
    },
}
```

### Configuration Example

```yaml
# crush-team.yaml
name: "Development Team"
description: "A team for software development tasks"
orchestrator: "magentic"  # AI-driven with ledgers
max_turns: 50
max_stalls: 3

agents:
  - name: "planner"
    role: "Task Planner"
    description: "Breaks down complex tasks into actionable steps"
    instructions: |
      You are a task planner. Analyze requests and create detailed plans.
      Consider dependencies and assign tasks to appropriate agents.
    cli: "crush"
    model: "glm-4-plus"
    handoffs: ["coder", "reviewer", "tester"]
    is_parallel: false

  - name: "coder"
    role: "Software Developer"
    description: "Implements code based on specifications"
    instructions: |
      You are a software developer. Write clean, tested code.
      Follow the plan and update facts as you discover things.
    cli: "crush"
    model: "claude-sonnet"
    tools: ["file_read", "file_write", "execute"]
    handoffs: ["reviewer", "planner"]
    is_parallel: true

  - name: "reviewer"
    role: "Code Reviewer"
    description: "Reviews code for quality and correctness"
    instructions: |
      You are a code reviewer. Check for bugs, style issues, and improvements.
      Provide actionable feedback.
    cli: "codex"
    handoffs: ["coder", "planner"]
    is_parallel: true

  - name: "tester"
    role: "Test Engineer"
    description: "Writes and runs tests"
    instructions: |
      You are a test engineer. Write comprehensive tests.
      Report any failures with clear reproduction steps.
    cli: "crush"
    model: "o3-mini"
    tools: ["execute", "file_write"]
    handoffs: ["coder"]
    is_parallel: true
```

### CLI Integration

```bash
# Run a team
crush team run crush-team.yaml "Implement a REST API for user authentication"

# With options
crush team run crush-team.yaml "task" \
  --orchestrator=dynamic \
  --max-turns=100 \
  --parallel \
  --watch

# Interactive mode
crush team interactive crush-team.yaml
```

### Implementation Phases

**Phase 1: Core Infrastructure**
- [ ] Agent definition types
- [ ] Agent registry + subprocess spawning
- [ ] Basic round-robin orchestrator

**Phase 2: AI-Driven Routing**
- [ ] Task ledger implementation
- [ ] Progress ledger implementation
- [ ] AI router with prompt engineering

**Phase 3: Advanced Patterns**
- [ ] Handoff tool integration
- [ ] Parallel execution
- [ ] Stall detection + re-planning

**Phase 4: Cross-CLI Support**
- [ ] Codex adapter
- [ ] Claude CLI adapter
- [ ] Gemini CLI adapter

**Phase 5: Polish**
- [ ] UI for team visualization
- [ ] Session persistence
- [ ] Export/import teams

---

## Key Differentiators from Other Frameworks

| Feature | Crush | OpenAI SDK | AutoGen | Letta |
|---------|-------|------------|---------|-------|
| **Subprocess isolation** | Yes | No | No | No |
| **Cross-CLI support** | Yes | No | No | No |
| **AI-decided routing** | Yes | Limited | Yes | Limited |
| **Ledger-based orchestration** | Yes | No | Yes (MagenticOne) | No |
| **No visual editor required** | Yes | Yes | No | Yes |
| **Sleep-time compute** | Planned | No | No | Yes |
| **Go implementation** | Yes | No | No | No |

---

## Next Steps

1. Review and refine this design
2. Implement Phase 1 core infrastructure
3. Create basic team configuration support
4. Test with 2-3 agent teams
5. Iterate on router prompts
