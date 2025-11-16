# ToolCallState Implementation Flow

> **Companion to**: `tool-call-state-architecture.md`
> **Purpose**: Document HOW the current implementation works (not what should be improved)
> **Audience**: Developers understanding/debugging the existing system

---

## Overview

This document provides comprehensive flow diagrams and call graphs showing how `ToolCallState` operates in the current implementation. For architectural critique and improvement proposals, see `tool-call-state-architecture.md`.

---

## State Machine Flow

```mermaid
stateDiagram-v2
    [*] --> Pending: Tool created

    Pending --> PermissionPending: Permission required
    Pending --> Running: Auto-approved/No permission

    PermissionPending --> PermissionApproved: User grants
    PermissionPending --> PermissionDenied: User denies

    PermissionApproved --> Running: Begin execution

    Running --> Completed: Success
    Running --> Failed: Error
    Running --> Cancelled: User cancels

    PermissionDenied --> [*]: Terminal
    Completed --> [*]: Terminal
    Failed --> [*]: Terminal
    Cancelled --> [*]: Terminal

    note right of Pending
        Initial state
        Icon: ⋯ (muted)
        Animation: Static
    end note

    note right of PermissionPending
        Awaiting approval
        Icon: ⋯ (paprika)
        Animation: Timer
    end note

    note right of Running
        Active execution
        Icon: ⋯ (green)
        Animation: Spinner
    end note

    note right of Completed
        Success
        Icon: ✓ (green)
        Animation: Blink
    end note
```

---

## Complete Call Flow: Agent → TUI

```mermaid
sequenceDiagram
    participant Agent as agent.go
    participant Perm as permission.go
    participant Msg as message.go
    participant Tool as tool.go (TUI)
    participant State as tool_call_state.go
    participant Anim as Animation

    Note over Agent: LLM requests tool execution

    %% Tool Creation
    Agent->>Msg: Create(ToolCall)<br/>State: Pending
    Agent->>Tool: OnToolInputStart(id, name)
    Tool->>State: call.State = Pending
    Tool->>Tool: configureVisualAnimation()
    Tool->>State: ToAnimationState()
    State-->>Tool: AnimationStateStatic
    Tool->>Anim: New(Static config)

    %% Permission Flow
    alt Requires Permission
        Agent->>Perm: Request(toolCallID, params)
        Perm->>Tool: Publish(PermissionPending)
        Tool->>State: call.State = PermissionPending
        Tool->>State: ToAnimationState()
        State-->>Tool: AnimationStateTimer
        Tool->>Anim: New(Timer config, paprika)

        alt User Approves
            Perm->>Tool: Publish(PermissionApproved)
            Tool->>State: call.State = PermissionApproved
            Tool->>State: ToAnimationState()
            State-->>Tool: AnimationStatePulse
            Tool->>Anim: New(Pulse config, green)
            Perm-->>Agent: true (approved)
        else User Denies
            Perm->>Tool: Publish(PermissionDenied)
            Tool->>State: call.State = PermissionDenied
            Tool->>State: ToAnimationState()
            State-->>Tool: AnimationStateStatic
            Tool->>Anim: New(Static config, error)
            Perm-->>Agent: false (denied)
            Note over Agent: Stop execution
        end
    end

    %% Execution Flow
    alt Execution Proceeds
        Agent->>Tool: OnToolCall(toolCall)
        Tool->>State: call.State = Running
        Tool->>State: ToAnimationState()
        State-->>Tool: AnimationStateSpinner
        Tool->>Anim: New(Spinner config, green cycling)

        Agent->>Agent: Execute tool logic

        alt Success
            Agent->>Tool: OnToolResult(result, isError: false)
            Tool->>Msg: Update Parts[i].State
            Msg->>Msg: State = Completed
            Tool->>State: ToAnimationState()
            State-->>Tool: AnimationStateBlink
            Tool->>Anim: New(Blink config)
        else Failure
            Agent->>Tool: OnToolResult(result, isError: true)
            Tool->>Msg: Update Parts[i].State
            Msg->>Msg: State = Failed
            Tool->>State: ToAnimationState()
            State-->>Tool: AnimationStateStatic
            Tool->>Anim: New(Static config, error)
        end
    end

    %% Final Rendering
    Tool->>State: ShouldShowContentForState(isNested, hasNested)
    State-->>Tool: bool (show/hide)
    Tool->>State: RenderTUIMessageColored()
    State-->>Tool: Styled message
```

---

## Layer-by-Layer Call Graph

### Layer 1: Agent Orchestration

```mermaid
graph TB
    subgraph "agent.go - Tool Execution"
        A1[PrepareStep] -->|Creates assistant msg| A2[OnToolInputStart]
        A2 -->|State: Pending| A3[message.ToolCall]
        A3 -->|Permission check| A4{Requires<br/>Permission?}

        A4 -->|Yes| A5[permission.Request]
        A4 -->|No| A6[OnToolCall]

        A5 -->|Approved| A6
        A5 -->|Denied| A7[HandleDenied]

        A6 -->|State: Running| A8[Execute Tool Logic]
        A8 -->|Result| A9[OnToolResult]

        A9 -->|Update state| A10{Result<br/>Type?}
        A10 -->|Success| A11[State: Completed]
        A10 -->|Error| A12[State: Failed]
        A10 -->|Cancel| A13[State: Cancelled]
    end

    style A1 fill:#e3f2fd
    style A6 fill:#c8e6c9
    style A11 fill:#a5d6a7
    style A12 fill:#ef9a9a
```

### Layer 2: Permission Management

```mermaid
graph TB
    subgraph "permission.go - Permission Flow"
        P1[permission.Request] -->|Create event| P2[PermissionRequest]
        P2 -->|Publish| P3[uiBroker.Publish<br/>PermissionPending]

        P3 -->|Wait for user| P4{User<br/>Decision?}

        P4 -->|Grant| P5[Grant/GrantPersistent]
        P4 -->|Deny| P6[Deny]

        P5 -->|Publish event| P7[Status: PermissionApproved]
        P6 -->|Publish event| P8[Status: PermissionDenied]

        P7 -->|Send to channel| P9[respCh <- Approved]
        P8 -->|Send to channel| P10[respCh <- Denied]
    end

    style P3 fill:#fff3e0
    style P5 fill:#c8e6c9
    style P6 fill:#ffcdd2
```

### Layer 3: TUI Component

```mermaid
graph TB
    subgraph "tool.go - UI Component"
        T1[NewToolCallCmp] -->|Initialize| T2[toolCallCmp]
        T2 -->|Has| T3[call: message.ToolCall]
        T2 -->|Has| T4[animationState: AnimationState]
        T2 -->|Has| T5[anim: util.Model]

        T6[SetToolCallState] -->|Update| T3
        T6 -->|Trigger| T7[RefreshAnimation]

        T7 -->|Calls| T8[configureVisualAnimation]
        T7 -->|Calls| T9[updateAnimationState]

        T8 -->|Read| T10[call.State]
        T8 -->|Switch on state| T11{State<br/>Type?}

        T11 -->|Pending| T12[Anim: Static, muted]
        T11 -->|PermissionPending| T13[Anim: Timer, paprika]
        T11 -->|PermissionApproved| T14[Anim: Pulse, green]
        T11 -->|Running| T15[Anim: Spinner, green]
        T11 -->|Final states| T16[Anim: Static/Blink]

        T9 -->|Calls| T17[getEffectiveDisplayState]
        T17 -->|Calls| T18[ToAnimationState]
    end

    style T2 fill:#e1f5fe
    style T8 fill:#fff9c4
```

### Layer 4: State Enum Methods

```mermaid
graph TB
    subgraph "tool_call_state.go - Enum Methods"
        E1[ToolCallState] -->|Methods| E2[9 separate methods]

        E2 --> M1[IsFinalState]
        E2 --> M2[ToIcon]
        E2 --> M3[ToFgColor]
        E2 --> M4[ToAnimationState]
        E2 --> M5[ShouldShowContentForState]
        E2 --> M6[RenderTUIMessage]
        E2 --> M7[RenderTUIMessageColored]
        E2 --> M8[ToIconColored]
        E2 --> M9[FormatToolForCopy]

        M1 -->|Returns bool| R1["Completed || Failed ||<br/>Cancelled || PermissionDenied"]
        M2 -->|Returns string| R2[Icon character]
        M3 -->|Returns color.Color| R3[Foreground color]
        M4 -->|Returns AnimationState| R4[Animation behavior]
        M5 -->|Returns bool| R5[Show/hide content]
        M6 -->|Returns string| R6[Status message]
        M7 -->|Returns string| R7[Styled message]
    end

    style E1 fill:#e8f5e9
    style E2 fill:#fff9c4
    style M4 fill:#e1f5fe
```

---

## State-Specific Behaviors

### Pending State

```go
// Location: tool_call_state.go:14-16
ToolCallStatePending ToolCallState = "pending"

// Animation: tool.go:873-881
anim.New(anim.Settings{
    Label:       "Waiting for tool to start...",
    GradColorA:  t.FgMuted,
    GradColorB:  t.FgMuted,
    CycleColors: false, // Static
})

// Content Visibility: tool_call_state.go:243-246
return hasNested && !isNested
// Only show if parent tool with nested calls
```

### Permission Pending State

```go
// Location: tool_call_state.go:18-19
ToolCallStatePermissionPending ToolCallState = "permission_pending"

// Animation: tool.go:883-891
anim.New(anim.Settings{
    Label:       "Awaiting permission...",
    GradColorA:  t.Paprika,
    GradColorB:  t.Paprika,
    CycleColors: false, // Timer animation
})

// Content Visibility: tool_call_state.go:223-224
return true // Show tool details while waiting
```

### Running State

```go
// Location: tool_call_state.go:27-28
ToolCallStateRunning ToolCallState = "running"

// Animation: tool.go:903-911
anim.New(anim.Settings{
    Label:       "Running...",
    GradColorA:  t.GreenDark,
    GradColorB:  t.Green,
    CycleColors: true, // Spinner animation
})

// Content Visibility: tool_call_state.go:248-249
return true // Show progress/running state
```

### Completed State

```go
// Location: tool_call_state.go:30-31
ToolCallStateCompleted ToolCallState = "completed"

// Animation: tool.go:912-922
anim.New(anim.Settings{
    Label:       "", // Empty for final states
    GradColorA:  t.FgMuted,
    CycleColors: false, // Blink briefly then static
})

// Message Rendering: tool_call_state.go:176-180
messageBaseStyle.Padding(0, 1)
    .Background(t.BgBaseLighter)
    .Foreground(t.FgSubtle)
    .Render("Done")
```

---

## Integration Points

### Agent → Permission

```go
// Location: agent.go:182
ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)

// Permission check happens in tool execution
// Tools call permission.Service.Request() internally
```

### Permission → TUI

```go
// Location: permission.go:88-96
func (s *permissionService) publishUnsafe(permission PermissionRequest, status enum.ToolCallState) {
    s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
        ToolCallID: permission.ToolCallID,
        Status:     status, // PermissionApproved/Denied
    })
    respCh, ok := s.pendingRequests.Get(permission.ID)
    if ok {
        respCh <- status
    }
}
```

### TUI → State Enum

```go
// Location: tool.go:816-820
func (m *toolCallCmp) updateAnimationState() {
    effectiveState := m.getEffectiveDisplayState()
    m.animationState = effectiveState.ToAnimationState()
}

// Location: tool.go:941-954
func (m *toolCallCmp) getEffectiveDisplayState() enum.ToolCallState {
    // Priority: Result > State
    if m.result.ToolCallID.IsNotEmpty() {
        if m.result.IsError {
            return enum.ToolCallStateFailed
        }
        return enum.ToolCallStateCompleted
    }
    return m.call.State
}
```

---

## State Transition Table

| From State | To State | Trigger | Location |
|------------|----------|---------|----------|
| `Pending` | `PermissionPending` | Permission required | `permission.go:113` |
| `Pending` | `Running` | Auto-approved | `agent.go:318` |
| `PermissionPending` | `PermissionApproved` | User grants | `permission.go:78` |
| `PermissionPending` | `PermissionDenied` | User denies | `permission.go:83` |
| `PermissionApproved` | `Running` | Begin execution | `agent.go:318` |
| `Running` | `Completed` | Success result | `agent.go:345-355` |
| `Running` | `Failed` | Error result | `agent.go:345-355` |
| `Running` | `Cancelled` | User cancels | `agent.go:467-468` |

---

## Method Reference

### IsFinalState()

```go
// Location: tool_call_state.go:40-45
func (state ToolCallState) IsFinalState() bool {
    return state == ToolCallStateCompleted ||
        state == ToolCallStateFailed ||
        state == ToolCallStateCancelled ||
        state == ToolCallStatePermissionDenied
}
```

**Purpose**: Determine if state is terminal (no further transitions)
**Used by**: `tool.go:434`, `agent.go:434`

### ToAnimationState()

```go
// Location: tool_call_state.go:188-217
func (state ToolCallState) ToAnimationState() AnimationState {
    switch state {
    case ToolCallStatePermissionPending:
        return AnimationStateTimer
    case ToolCallStatePermissionApproved:
        return AnimationStatePulse
    case ToolCallStateRunning:
        return AnimationStateSpinner
    case ToolCallStateCompleted:
        return AnimationStateBlink
    // ... other states → Static/None
    }
}
```

**Purpose**: Map tool state to animation behavior
**Used by**: `tool.go:819`

### ShouldShowContentForState()

```go
// Location: tool_call_state.go:220-256
func (state ToolCallState) ShouldShowContentForState(isNested, hasNested bool) bool {
    // Permission states: show tool details
    // Pending: only show if parent with nested calls
    // Running/Final: show content
    // PermissionDenied: hide content
}
```

**Purpose**: Control content visibility based on state and nesting
**Used by**: `renderer.go:107`, `renderer.go:925`
**Parameters**:
- `isNested`: This tool is nested inside another
- `hasNested`: This tool has nested children

---

## Performance Characteristics

### State Checks (Hot Path)

```go
// Called on every render frame (~20 FPS when animating)
IsAnimating() -> animationState.IsActive() -> O(1)

// Called on state change only
ToAnimationState() -> switch statement -> O(1)
```

### State Transitions

```go
// Typical flow timing:
Pending -> Running: ~0-100ms (permission check)
Running -> Completed: Variable (tool execution time)
PermissionPending -> Approved: User-dependent (seconds to minutes)
```

---

## Common Patterns

### Pattern 1: Check and Update State

```go
// agent.go:343-357
for i, part := range currentAssistant.Parts {
    if tc, ok := part.(message.ToolCall); ok && tc.ID == result.ToolCallID {
        newState := enum.ToolCallStateCompleted
        if isError {
            newState = enum.ToolCallStateFailed
        }
        currentAssistant.Parts[i] = message.ToolCall{
            ID:    tc.ID,
            Name:  tc.Name,
            Input: tc.Input,
            State: newState,
        }
        break
    }
}
```

### Pattern 2: State-Aware Rendering

```go
// renderer.go:106-113
if v.call.State.ShouldShowContentForState(v.isNested, len(v.nestedToolCalls) > 0) {
    body := contentRenderer()
    return joinHeaderBody(header, body)
}
return header
```

### Pattern 3: Animation Configuration

```go
// tool.go:858-923
switch m.call.State {
case enum.ToolCallStatePending:
    m.anim = anim.New(anim.Settings{...})
case enum.ToolCallStateRunning:
    m.anim = anim.New(anim.Settings{...})
// ... other states
}
```

---

## Debugging Tips

### 1. State Not Updating

```go
// Check: Is state being set correctly?
log.Debug("Tool state transition",
    "toolID", tc.ID,
    "from", oldState,
    "to", newState)

// Check: Is RefreshAnimation() being called?
// Location: tool.go:852
```

### 2. Wrong Animation

```go
// Check: What's the effective display state?
effectiveState := m.getEffectiveDisplayState()
log.Debug("Animation state",
    "callState", m.call.State,
    "effectiveState", effectiveState,
    "animState", effectiveState.ToAnimationState())
```

### 3. Content Not Showing

```go
// Check: What does ShouldShowContentForState return?
shouldShow := m.call.State.ShouldShowContentForState(m.isNested, len(m.nestedToolCalls) > 0)
log.Debug("Content visibility",
    "state", m.call.State,
    "isNested", m.isNested,
    "hasNested", len(m.nestedToolCalls) > 0,
    "shouldShow", shouldShow)
```

---

## See Also

- **Architecture Critique**: `tool-call-state-architecture.md` - Why this should be refactored
- **Animation Flow**: `animation-state-flow.md` - How animations work
- **Implementation**: `internal/enum/tool_call_state.go` - Source code
- **Tests**: `internal/enum/tool_call_state_test.go` - Test coverage

---

**Document Purpose**: Implementation reference (not architectural proposal)
**Last Updated**: 2025-11-16
**Maintenance**: Update when state machine logic changes
