# AnimationState Implementation Flow

> **Companion to**: `animation-state-architecture.md`
> **Purpose**: Document HOW the animation system works (not improvement proposals)
> **Audience**: Developers understanding/debugging the animation system

---

## Overview

This document provides comprehensive flow diagrams showing how `AnimationState` controls visual animations in the current implementation. For architectural enhancements and proposals, see `animation-state-architecture.md`.

---

## Animation State Lifecycle

```mermaid
stateDiagram-v2
    [*] --> None: Unknown/Default

    None --> Static: No animation needed
    None --> Spinner: Active tool execution
    None --> Timer: Awaiting permission
    None --> Blink: Success feedback
    None --> Pulse: Transitional state

    Static --> Spinner: Tool starts
    Static --> Timer: Permission needed

    Timer --> Pulse: Permission approved
    Timer --> Static: Permission denied

    Pulse --> Spinner: Begin execution
    Pulse --> Static: Cancelled

    Spinner --> Blink: Quick completion
    Spinner --> Static: Error/Cancel

    Blink --> Static: Animation complete

    Static --> [*]: Final state

    note right of None
        No visual animation
        Empty string value
    end note

    note right of Static
        Static display
        No cycling colors
        CycleColors: false
    end note

    note right of Spinner
        Dot spinner
        20 FPS cycling
        Green gradient
    end note

    note right of Timer
        Countdown timer
        20 FPS cycling
        Paprika color
    end note

    note right of Blink
        Brief success blink
        Then becomes static
    end note

    note right of Pulse
        Pulsing animation
        Transitional feedback
        Green gradient
    end note
```

---

## Complete Animation Flow

```mermaid
sequenceDiagram
    participant Tool as toolCallCmp
    participant State as ToolCallState
    participant AnimState as AnimationState
    participant Anim as anim.Anim
    participant Loop as TUI Loop

    Note over Tool: Tool state changes

    Tool->>Tool: SetToolCallState(newState)
    Tool->>Tool: RefreshAnimation()

    Tool->>Tool: updateAnimationState()
    Tool->>State: getEffectiveDisplayState()
    State-->>Tool: Effective state
    Tool->>State: ToAnimationState()
    State-->>AnimState: Returns AnimationState enum

    Tool->>Tool: configureVisualAnimation()
    Tool->>State: Read call.State

    alt Pending State
        Tool->>Anim: New(Settings{<br/>  Size: 15,<br/>  Label: "Waiting...",<br/>  GradColorA: FgMuted,<br/>  CycleColors: false<br/>})
        Note over Anim: AnimationStateStatic
    else Permission Pending
        Tool->>Anim: New(Settings{<br/>  Size: 15,<br/>  Label: "Awaiting permission...",<br/>  GradColorA: Paprika,<br/>  CycleColors: false<br/>})
        Note over Anim: AnimationStateTimer
    else Running
        Tool->>Anim: New(Settings{<br/>  Size: 15,<br/>  Label: "Running...",<br/>  GradColorA: GreenDark,<br/>  GradColorB: Green,<br/>  CycleColors: true<br/>})
        Note over Anim: AnimationStateSpinner
    else Completed
        Tool->>Anim: New(Settings{<br/>  Size: 15,<br/>  Label: "",<br/>  CycleColors: false<br/>})
        Note over Anim: AnimationStateBlink
    end

    Anim->>Anim: Generate gradient ramp
    Anim->>Anim: Pre-render all frames
    Anim->>Anim: Cache results
    Anim-->>Tool: Anim instance

    Tool->>Anim: Init()
    Anim-->>Loop: Step Cmd (20 FPS)

    loop Every 50ms (20 FPS)
        Loop->>Tool: StepMsg
        Tool->>Tool: IsAnimating()
        Tool->>AnimState: IsActive()

        alt Animation Active
            AnimState-->>Tool: true
            Tool->>Anim: Update(StepMsg)
            Anim->>Anim: step.Add(1)
            Anim->>Anim: ellipsisStep.Add(1)
            Anim-->>Tool: Next Step Cmd

            Loop->>Tool: Render View()
            Tool->>Anim: View()
            Anim->>Anim: Render frame[step]
            Anim-->>Tool: Rendered string
        else Animation Static
            AnimState-->>Tool: false
            Tool->>Loop: Skip update
        end
    end
```

---

## Layer-by-Layer Flow

### Layer 1: State to Animation Mapping

```mermaid
graph TB
    subgraph "ToolCallState → AnimationState"
        TS1[ToolCallState] -->|ToAnimationState| MAP{Mapping<br/>Logic}

        MAP -->|Pending| AnimationStateStatic[AnimationStateStatic]
        MAP -->|PermissionPending| AnimationStateTimer[AnimationStateTimer]
        MAP -->|PermissionApproved| AnimationStatePulse[AnimationStatePulse]
        MAP -->|PermissionDenied| AnimationStateStatic[AnimationStateStatic]
        MAP -->|Running| AnimationStateSpinner[AnimationStateSpinner]
        MAP -->|Completed| AnimationStateBlink[AnimationStateBlink]
        MAP -->|Failed| AnimationStateStatic[AnimationStateStatic]
        MAP -->|Cancelled| AnimationStateStatic[AnimationStateStatic]
        MAP -->|Unknown| AnimationStateNone[AnimationStateNone]
    end

    subgraph "Animation Characteristics"
        AnimationStateStatic -->|No cycling| C1[Muted gray, static]
        AnimationStateTimer -->|Timer| C2[Paprika, no cycling]
        AnimationStatePulse -->|Pulse| C3[Green gradient, cycling]
        AnimationStateStatic -->|Error| C4[Error color, static]
        AnimationStateSpinner -->|Spinner| C5[Green gradient, cycling]
        AnimationStateBlink -->|Blink| C6[Brief animation]
        AnimationStateStatic -->|Error| C7[Error color, static]
        AnimationStateStatic -->|Muted| C8[Muted gray, static]
        AnimationStateNone -->|None| C9[No display]
    end

    style AnimationStateStatic fill:#E0E0E0
    style AnimationStateTimer fill:#FFA07A
    style AnimationStatePulse fill:#90EE90
    style AnimationStateStatic fill:#FFB6C1
    style AnimationStateSpinner fill:#87CEEB
    style AnimationStateBlink fill:#98FB98
    style AnimationStateNone fill:#FFFFFF
```

### Layer 2: Animation Component

```mermaid
graph TB
    subgraph "anim.Anim - Animation Engine"
        A1[New Settings] -->|Initialize| A2[Anim struct]

        A2 -->|Contains| F1[step: atomic.Int64]
        A2 -->|Contains| F2[ellipsisStep: atomic.Int64]
        A2 -->|Contains| F3["initialFrames: [][]string"]
        A2 -->|Contains| F4["cyclingFrames: [][]string"]
        A2 -->|Contains| F5["label: []string"]
        A2 -->|Contains| F6["birthOffsets: []duration"]

        A3[Init] -->|Returns| A4[Step Cmd]
        A4 -->|Schedule| A5[StepMsg every 50ms]

        A6[Update] -->|Receives| A5
        A6 -->|Increment| F1
        A6 -->|Update| F2
        A6 -->|Returns| A4

        A7[View] -->|Read| F1
        A7 -->|Select| F3
        A7 -->|Or| F4
        A7 -->|Append| F5
        A7 -->|Returns| A8[Rendered string]
    end

    style A2 fill:#e1f5fe
    style A4 fill:#fff9c4
    style A8 fill:#c8e6c9
```

### Layer 3: Frame Generation

```mermaid
graph TB
    subgraph "Frame Pre-rendering (anim.go:New)"
        G1[New Animation] -->|Check cache| G2{Cache<br/>Hit?}

        G2 -->|Yes| G3[Load from cache]
        G2 -->|No| G4[Generate frames]

        G4 -->|Create| G5[Gradient ramp]
        G5 -->|Number of colors| G6["width × 3 (cycling)<br/>width (static)"]

        G6 -->|Pre-render| G7[initialFrames]
        G6 -->|Pre-render| G8[cyclingFrames]

        G7 -->|For each frame| G9[Render '.' with color]
        G8 -->|For each frame| G10[Render random rune with color]

        G10 -->|Cache| G11[animCacheMap.Set]

        G3 -->|Use| G12[Cached frames]
        G11 -->|Store| G12
    end

    style G5 fill:#fff3e0
    style G11 fill:#c8e6c9
```

---

## Animation Configuration Details

### Static Animation (Pending, Failed, Cancelled, Denied)

```go
// Location: tool.go:873-881, 912-922
anim.New(anim.Settings{
    Size:        15,              // Number of chars
    Label:       "Waiting...",    // Or "" for final states
    GradColorA:  t.FgMuted,       // Muted gray
    GradColorB:  t.FgMuted,       // Same color (no gradient)
    CycleColors: false,           // No animation
})

// Result:
// - No color cycling
// - Static dots (no random runes)
// - Label may have ellipsis animation (... .. . ...)
```

**Visual**: `⋯ Waiting...`

### Timer Animation (Permission Pending)

```go
// Location: tool.go:883-891
anim.New(anim.Settings{
    Size:        15,
    Label:       "Awaiting permission...",
    GradColorA:  t.Paprika,       // Orange/paprika color
    GradColorB:  t.Paprika,       // Same color
    CycleColors: false,           // No color cycling (timer-specific)
})

// Result:
// - Paprika colored dots
// - Ellipsis animates every 400ms
// - No color cycling (special case)
```

**Visual**: `⋯ Awaiting permission...` (paprika colored)

### Spinner Animation (Running)

```go
// Location: tool.go:903-911
anim.New(anim.Settings{
    Size:        15,
    Label:       "Running...",
    GradColorA:  t.GreenDark,     // Dark green
    GradColorB:  t.Green,         // Bright green
    CycleColors: true,            // Enable cycling!
})

// Result:
// - Green gradient from dark to bright
// - Colors cycle/rotate every 50ms
// - Random runes cycle
// - Ellipsis animates
```

**Visual**: `⋯ Running...` (animated green gradient)

### Pulse Animation (Permission Approved)

```go
// Location: tool.go:893-901
anim.New(anim.Settings{
    Size:        15,
    Label:       "Permission approved. Executing...",
    GradColorA:  t.GreenDark,
    GradColorB:  t.Green,
    CycleColors: true,            // Pulsing effect
})

// Result:
// - Similar to spinner but transitional
// - Brief state before Running
```

**Visual**: `⋯ Permission approved...` (pulsing green)

### Blink Animation (Completed)

```go
// Location: tool.go:912-922
anim.New(anim.Settings{
    Size:        15,
    Label:       "",              // Empty label for final states
    GradColorA:  t.FgMuted,
    GradColorB:  t.FgMuted,
    CycleColors: false,
})

// Note: Blink is currently implemented as static
// TODO: Implement actual brief blink animation
```

**Visual**: Static (no label)

---

## Animation State Methods

### IsActive()

```go
// Location: animation_state.go:33-38
func (state AnimationState) IsActive() bool {
    return state == AnimationStateSpinner ||
           state == AnimationStateTimer ||
           state == AnimationStateBlink ||
           state == AnimationStatePulse
}
```

**Purpose**: Determine if animation should update every frame
**Used by**: `tool.go:824-832`
**Performance**: O(1) comparison

### IsStatic()

```go
// Location: animation_state.go:41-43
func (state AnimationState) IsStatic() bool {
    return state == AnimationStateNone || state == AnimationStateStatic
}
```

**Purpose**: Determine if animation should not move
**Used by**: Rendering logic
**Note**: `!IsActive()` might be more accurate

### ToIcon()

```go
// Location: animation_state.go:51-64
func (state AnimationState) ToIcon() string {
    switch state {
    case AnimationStateNone, AnimationStateStatic:
        return ""
    case AnimationStateSpinner, AnimationStateTimer:
        return "⋯" // Loading dots
    case AnimationStateBlink:
        return "✅" // Success checkmark
    case AnimationStatePulse:
        return "⚡" // Lightning bolt
    default:
        return ""
    }
}
```

**Purpose**: Get static icon representation
**Note**: Not currently used (animations render dynamically)

---

## Frame Rendering Pipeline

### Step 1: New Animation Creation

```go
// Location: anim.go:122-255
func New(opts Settings) *Anim {
    // 1. Validate settings
    if opts.Size < 1 { opts.Size = defaultNumCyclingChars }

    // 2. Check cache
    cacheKey := settingsHash(opts)
    if cached, exists := animCacheMap.Get(cacheKey); exists {
        // Use cached frames
        a.initialFrames = cached.initialFrames
        a.cyclingFrames = cached.cyclingFrames
        // ...
        return a
    }

    // 3. Generate gradient ramp
    numFrames := prerenderedFrames // 10 for static
    if opts.CycleColors {
        numFrames = a.width * 2 // More for cycling
    }
    ramp := makeGradientRamp(size, colorA, colorB, ...)

    // 4. Pre-render initial frames (dots)
    for i := range numFrames {
        for j := range width {
            color := ramp[j+offset]
            a.initialFrames[i][j] = lipgloss.NewStyle()
                .Foreground(color)
                .Render(".")
        }
    }

    // 5. Pre-render cycling frames (random runes)
    for i := range numFrames {
        for j := range width {
            color := ramp[j+offset]
            rune := randomRune()
            a.cyclingFrames[i][j] = lipgloss.NewStyle()
                .Foreground(color)
                .Render(string(rune))
        }
    }

    // 6. Cache results
    animCacheMap.Set(cacheKey, cached)

    return a
}
```

### Step 2: Frame Update (20 FPS)

```go
// Location: anim.go:322-348
func (a *Anim) Update(msg tea.Msg) (util.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case StepMsg:
        // 1. Verify message is for this instance
        if msg.id != a.id {
            return a, nil
        }

        // 2. Increment frame counter
        step := a.step.Add(1)
        if int(step) >= len(a.cyclingFrames) {
            a.step.Store(0) // Wrap around
        }

        // 3. Update ellipsis animation (every 8 frames = 400ms)
        if a.initialized.Load() && a.labelWidth > 0 {
            ellipsisStep := a.ellipsisStep.Add(1)
            if int(ellipsisStep) >= ellipsisAnimSpeed*len(ellipsisFrames) {
                a.ellipsisStep.Store(0)
            }
        }

        // 4. Check if birth phase complete
        if !a.initialized.Load() && time.Since(a.startTime) >= maxBirthOffset {
            a.initialized.Store(true)
        }

        return a, a.Step() // Schedule next frame
    }
}
```

### Step 3: Frame Rendering

```go
// Location: anim.go:351-383
func (a *Anim) View() string {
    var b strings.Builder
    step := int(a.step.Load())

    for i := range a.width {
        switch {
        // Phase 1: Birth animation (first 1 second)
        case !a.initialized.Load() && time.Since(a.startTime) < a.birthOffsets[i]:
            b.WriteString(a.initialFrames[step][i]) // Static dot

        // Phase 2: Cycling characters
        case i < a.cyclingCharWidth:
            b.WriteString(a.cyclingFrames[step][i]) // Animated character

        // Phase 3: Label gap
        case i == a.cyclingCharWidth:
            b.WriteString(labelGap) // Space

        // Phase 4: Label text
        case i > a.cyclingCharWidth:
            if labelChar, ok := a.label.Get(i - a.cyclingCharWidth - labelGapWidth); ok {
                b.WriteString(labelChar)
            }
        }
    }

    // Phase 5: Ellipsis animation (... .. . ...)
    if a.initialized.Load() && a.labelWidth > 0 {
        ellipsisStep := int(a.ellipsisStep.Load())
        if ellipsisFrame, ok := a.ellipsisFrames.Get(ellipsisStep / ellipsisAnimSpeed); ok {
            b.WriteString(ellipsisFrame)
        }
    }

    return b.String()
}
```

---

## Performance Characteristics

### Frame Rate

```go
const fps = 20
// Frame time: 1000ms / 20 = 50ms per frame
```

**Implications**:
- Update loop runs every 50ms
- Max 20 visual updates per second
- Smooth enough for terminal animations

### Pre-rendering

```go
// Static animation:
prerenderedFrames = 10
// Memory: 10 frames × 15 chars × ~4 bytes = ~600 bytes

// Cycling animation:
prerenderedFrames = a.width * 2 = 15 * 2 = 30
// Memory: 30 frames × 15 chars × ~4 bytes = ~1.8 KB
```

**Benefits**:
- Zero runtime computation
- Consistent frame time
- No jank or stuttering

### Caching

```go
// Cache key: hash of (Size, Label, Colors, CycleColors)
// Cache hit rate: >95% (most tools use same configs)
// Memory saved: ~1-2 KB per cache hit
```

**Global cache size**: ~10-20 entries = ~10-20 KB total

### Atomic Operations

```go
step.Add(1)          // Lock-free increment
step.Load()          // Lock-free read
step.Store(0)        // Lock-free write
```

**Benefits**:
- Thread-safe without mutexes
- No contention
- Predictable performance

---

## Integration with ToolCallState

### Mapping Logic

```go
// Location: tool_call_state.go:188-217
func (state ToolCallState) ToAnimationState() AnimationState {
    switch state {
    case ToolCallStatePermissionPending:
        return AnimationStateTimer
    case ToolCallStatePermissionApproved:
        return AnimationStatePulse
    case ToolCallStatePermissionDenied, ToolCallStateFailed, ToolCallStateCancelled:
        return AnimationStateStatic
    case ToolCallStateCompleted:
        return AnimationStateBlink
    case ToolCallStateRunning:
        return AnimationStateSpinner
    case ToolCallStatePending:
        return AnimationStateStatic
    default:
        return AnimationStateNone
    }
}
```

### Update Trigger

```go
// Location: tool.go:807-812
func (m *toolCallCmp) RefreshAnimation() {
    m.configureVisualAnimation()  // Update visual component
    m.updateAnimationState()       // Update state enum
}

// Called by:
// - SetToolCallState()
// - NewToolCallCmp()
// - Permission events
```

---

## Animation Lifecycle

```mermaid
graph TB
    START[Tool Created] -->|NewToolCallCmp| INIT[Init Animation]
    INIT -->|New Settings| GEN[Generate Frames]
    GEN -->|Check cache| CACHE{Cache<br/>Hit?}

    CACHE -->|Yes| LOAD[Load Cached]
    CACHE -->|No| RENDER[Pre-render All Frames]
    RENDER -->|Store| SAVE[Save to Cache]
    SAVE --> READY[Animation Ready]
    LOAD --> READY

    READY -->|Init| LOOP[Start Update Loop]
    LOOP -->|Every 50ms| UPDATE[Update Frame]
    UPDATE -->|Increment step| WRAP{Wrap<br/>Around?}
    WRAP -->|Yes| RESET[step = 0]
    WRAP -->|No| CONTINUE[step++]
    RESET --> RENDER_FRAME[Render Frame]
    CONTINUE --> RENDER_FRAME

    RENDER_FRAME -->|View| DISPLAY[Display to User]
    DISPLAY -->|Next frame| LOOP

    CHANGE[State Changes] -->|RefreshAnimation| RECONFIG[Reconfigure Animation]
    RECONFIG -->|New settings| GEN

    FINAL[Tool Final State] -->|Stop cycling| STATIC[Static Display]
    STATIC -->|IsActive = false| END[No More Updates]

    style GEN fill:#fff9c4
    style CACHE fill:#e1f5fe
    style RENDER fill:#fff3e0
    style LOOP fill:#c8e6c9
```

---

## Common Patterns

### Pattern 1: Create Animation

```go
// tool.go:858-939
switch m.call.State {
case enum.ToolCallStatePending:
    m.anim = anim.New(anim.Settings{
        Size:        15,
        Label:       label,
        GradColorA:  t.FgMuted,
        GradColorB:  t.FgMuted,
        CycleColors: false,
    })
// ... other cases
}
```

### Pattern 2: Check if Animating

```go
// tool.go:823-833
func (m *toolCallCmp) IsAnimating() bool {
    if m.animationState.IsActive() {
        return true
    }
    for _, nested := range m.nestedToolCalls {
        if nested.IsAnimating() {
            return true
        }
    }
    return false
}
```

### Pattern 3: Render Animation

```go
// tool.go:152-154, 725-736
func (m *toolCallCmp) renderState() string {
    icon := m.call.State.ToIconColored()
    tool := prettifyToolName(m.call.Name)
    return fmt.Sprintf("%s %s %s", icon, tool, m.anim.View())
}
```

---

## Debugging Tips

### 1. Animation Not Showing

```go
// Check: Is animation active?
log.Debug("Animation state",
    "animState", m.animationState,
    "isActive", m.animationState.IsActive(),
    "callState", m.call.State)

// Check: Is View() being called?
animView := m.anim.View()
log.Debug("Animation view", "output", animView)
```

### 2. Wrong Animation Type

```go
// Check: State to animation mapping
toolState := m.call.State
animState := toolState.ToAnimationState()
log.Debug("Animation mapping",
    "toolState", toolState,
    "animState", animState,
    "expected", "spinner/timer/static")
```

### 3. Performance Issues

```go
// Check: Frame rate
startTime := time.Now()
// ... render frame ...
duration := time.Since(startTime)
if duration > 50*time.Millisecond {
    log.Warn("Slow frame", "duration", duration)
}

// Check: Cache hit rate
hits, misses := animCacheMap.Stats()
hitRate := float64(hits) / float64(hits+misses)
log.Info("Cache stats", "hitRate", hitRate)
```

---

## See Also

- **Architecture Enhancements**: `animation-state-architecture.md` - Proposed improvements
- **Tool State Flow**: `tool-call-state-flow.md` - How tool states work
- **Implementation**: `internal/enum/animation_state.go` - Source code
- **Animation Engine**: `internal/tui/components/anim/anim.go` - Animation system

---

**Document Purpose**: Implementation reference (not architectural proposal)
**Last Updated**: 2025-11-16
**Maintenance**: Update when animation logic changes
