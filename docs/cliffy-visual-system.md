# Cliffy Visual Task Processing Methodology

## Overview

This document defines the visual language for Cliffy's task processing system using Unicode symbols. The design creates a distinctive "Cliffy look" that communicates task states, data flow, and system behavior through a tennis/volley metaphor.

## Design Philosophy

1. **Tennis/Volley Metaphor**: Tasks are "volleyed" between workers like tennis balls
2. **Process Visualization**: Show tasks progressing through stages (queued → processing → complete)
3. **Data Flow**: Illustrate information moving through the system
4. **At-a-Glance Status**: Users should instantly understand system state
5. **Terminal-Friendly**: All symbols render correctly in standard terminals

## Current Symbols (Existing)

### Task States
- `○` (U+25CB) - Hollow circle: Queued task, waiting to start
- `◴◵◶◷` (U+25F4-F7) - Quarter spinners: Running task animation
- `●` (U+25CF) - Solid circle: Completed task

### Tool States
- `▣` (U+25A3) - Filled square: Tool succeeded
- `☒` (U+2612) - Checked box: Tool failed

### Tree Structure
- `╮` (U+256E) - Down-right corner: Task branches to tools
- `├` (U+251C) - Tree mid: Middle tool in list
- `╰` (U+2570) - Up-right corner: Last tool in list
- `───` (U+2500×3) - Horizontal line: Connector

### Branding
- `◍` (U+25CD) - Shadowed circle: Tennis racket head
- `ᕕ( ᐛ )ᕗ` - Cliffy character

## Enhanced Symbol System

### Task Lifecycle States

**Processing Phases** (filling circles show progress):
```
○  (U+25CB)  Queued       - 0% empty, waiting in queue
◔  (U+25D4)  Initializing - 25% filled, task starting up
◑  (U+25D1)  Processing   - 50% filled, actively working
◕  (U+25D5)  Finalizing   - 75% filled, wrapping up
●  (U+25CF)  Complete     - 100% filled, done
◌  (U+25CC)  Failed       - Broken/dotted, task failed
⦿  (U+29BF)  Canceled     - Ringed dot, task canceled
◉  (U+25C9)  Cached       - Double circle, cached result
```

**Animation Frames** (keep existing for compatibility):
```
◴◵◶◷  (U+25F4-F7)  Spinner - Alternative running animation
⠁⠂⠄⡀⢀⠠⠐⠈  (U+2801+)  Braille dots - Smooth spinner (optional)
```

### Agent/Worker States

**Worker Status**:
```
⬡  (U+2B21)  Idle         - Hexagon, worker ready
⬢  (U+2B22)  Active       - Filled hexagon, worker processing
⬣  (U+2B23)  Overloaded   - Black hexagon, worker at capacity
⎔  (U+2394)  Suspended    - Squared hexagon, worker paused
```

**Concurrency Visualization**:
```
║  (U+2551)  Worker lane  - Vertical double line
═  (U+2550)  Queue line   - Horizontal double line
╬  (U+256C)  Junction     - Worker/queue intersection
```

### Data Flow & Processing

**Flow Indicators**:
```
→  (U+2192)  Simple flow       - Basic direction
⇒  (U+21D2)  Strong flow       - Emphasized direction
⇨  (U+21E8)  Fast flow         - Rapid processing
⟶  (U+27F6)  Long arrow        - Extended pipeline
↦  (U+21A6)  Maps to           - Transformation
⇄  (U+21C4)  Bidirectional     - Request/response
⟲  (U+27F2)  Retry loop        - Task retry
⟳  (U+27F3)  Refresh           - Cache refresh
↻  (U+21BB)  Feedback          - Tool feedback
⤴  (U+2934)  Return            - Error return
```

**Container/Blueprint States**:
```
□  (U+25A1)  Blueprint         - Task template
▢  (U+25A2)  Container         - Task context
▣  (U+25A3)  Filled block      - Tool succeeded (existing)
▤  (U+25A4)  Top fill          - Initializing
▥  (U+25A5)  Right fill        - Processing
▦  (U+25A6)  Grid fill         - Complex processing
▧  (U+25A7)  Diagonal fill     - Transforming
▨  (U+25A8)  Dense fill        - Heavy processing
▩  (U+25A9)  Crossed fill      - Error state
▪  (U+25AA)  Small block       - Micro-task
▫  (U+25AB)  Small hollow      - Pending micro-task
```

**System States**:
```
█  (U+2588)  Solid unit        - Operational component
▮  (U+25AE)  Output marker     - Result ready
▯  (U+25AF)  Input marker      - Awaiting input
▬  (U+25AC)  Buffer            - Queued items
▭  (U+25AD)  Empty buffer      - Empty queue
```

### Advanced Visualizations

**Machine/Assembler Nodes**:
```
╱  (U+2571)  Left diagonal     - Input path
╲  (U+2572)  Right diagonal    - Output path
╳  (U+2573)  Crossed           - Processing node
┼  (U+253C)  Cross junction    - Router/splitter
◇  (U+25C7)  Diamond           - Decision point
◆  (U+25C6)  Filled diamond    - Active decision
```

**Error & Warning States**:
```
⚠  (U+26A0)  Warning           - Retry needed
✗  (U+2717)  Error             - Failed
⊗  (U+2297)  Blocked           - Cannot proceed
⊘  (U+2298)  Prohibited        - Invalid operation
⚡  (U+26A1)  Rate limited      - Too fast
⏸  (U+23F8)  Paused            - Waiting
```

**Metrics & Status**:
```
⟨⟩ (U+27E8-9) Angle brackets   - Metrics container
⌈⌉ (U+2308-9) Ceiling brackets - Upper bound
⌊⌋ (U+230A-B) Floor brackets   - Lower bound
⋯  (U+22EF)  Midline ellipsis  - Truncated
⋮  (U+22EE)  Vertical ellipsis - More items
```

## Visual Layout Patterns

### Header (Volley Start)

```
◍═══╕  3 tasks volleyed
    ╰──╮ Using x-ai/grok-4-fast
```

### Task Display Variants

**Simple Task (no tools)**:
```
1   ● analyze auth.go (1.2s, 2.3k tokens)
```

**Task with Tools (expanded)**:
```
1 ╮ ◑ refactor authentication system (worker 1)
  ├───▣ read     auth.go  0.3s
  ├───▣ grep     findUser.*  0.1s
  ╰───▣ edit     auth.go  0.2s
```

**Task with Tools (collapsed)**:
```
1 ╮ ● refactor auth [read grep edit]  3.2k tokens $0.0042  1.8s
```

### Enhanced Progress Display

**With Processing States**:
```
◍═══╕  5 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze auth.go [read grep]  2.1k tokens $0.0021  1.2s
2 ╮ ◕ refactor database layer (worker 1)
  ├───▣ read     db.go  0.4s
  ╰───▤ edit     db.go  processing...
3   ◑ update tests (worker 2)
4   ◔ fix linting errors (worker 3)
5   ○ build project
```

**With Worker Visualization**:
```
◍═══╕  5 tasks volleyed ⬢⬢⬢ (3 workers)
    ╰──╮ Using claude-3-5-sonnet

Worker ⬢1 → 2 ╮ ◕ refactor database
Worker ⬢2 → 3   ◑ update tests
Worker ⬢3 → 4   ◔ fix linting
Queue      → 5   ○ build project
```

### Retry & Error States

**Retry Loop**:
```
2   ◌ authentication check (attempt 2) ⟲
    ⤴ Error: rate limited (429)
    ⏸ Retrying in 2.0s...
```

**Failed Task**:
```
3 ╮ ◌ deploy to production ✗
  ├───▣ bash     git push  0.5s
  ╰───▩ bash     deploy.sh  ⊗ exit 1
    ⤴ Error: deployment failed
```

### Footer (Volley Complete)

**Success**:
```
◍ 5/5 tasks succeeded in 8.3s
  15.2k tokens  $0.0187  ⬢⬢⬢ (3 workers used)
```

**Mixed Results**:
```
◍ 3/5 succeeded, 2 failed in 12.1s
  ✓ Tasks: 1, 2, 5
  ✗ Tasks: 3, 4
  8.7k tokens  $0.0104
```

## Data Flow Diagrams

### Task Pipeline

```
Queue   Worker Pool    Agent       Provider    Result
═══     ║  ║  ║       ╱╲          ⟨API⟩       ═══
 □  ⇨   ⬢  ⬡  ⬡  ⇨   ╳   ⇨  ⟶              ⇨  ●
 □  ⇨   ⬡  ⬢  ⬡  ⇨   ╱╲  ⇨  ⟶   request    ⇨  ●
 □      ⬡  ⬡  ⬢       ╳      ⟶   response      ●
 ○      ║  ║  ║       ╱╲         ⤴ retry       ○
```

### Volley Execution Flow

```
Input → Scheduler → Workers → Agent → Tools → Output
 □□□  ⇨   ╬╬╬    ⇨  ⬢⬢⬢  ⇨  ╱╲  ⇨  ▣▣▣  ⇨  ●●●
```

### Retry Mechanism

```
Task ⇨ Agent ⇨ Provider
  ○           ⟨API⟩
  ↓             ⤴
  ◔           ⚡ 429
  ↓             ↓
  ◌ ⟲ ⏸ 2s → ◔ ⇨ retry
  ↓
  ●
```

## Implementation Locations

### Current Files
- `/internal/llm/tools/ascii.go` - Symbol constants
- `/internal/volley/progress.go` - Progress rendering
- `/internal/volley/task.go` - Task states
- `/internal/llm/agent/agent.go` - Agent events

### Proposed New Files
- `/internal/llm/tools/symbols.go` - Enhanced symbol library
- `/internal/volley/visualizer.go` - Advanced visualization logic
- `/internal/volley/layout.go` - Layout patterns and templates

## Symbol Usage Guidelines

### When to Use Each State

**Task States**:
- `○` Queued: Task in queue, not started
- `◔` Initializing: Agent starting, loading context (0-25% complete)
- `◑` Processing: Agent actively working, tools executing (25-75%)
- `◕` Finalizing: Agent completing, formatting output (75-99%)
- `●` Complete: Task finished successfully
- `◌` Failed: Task encountered fatal error
- `⦿` Canceled: Task canceled by user or fail-fast

**Tool States**:
- `▤` Starting: Tool initializing
- `▥` Running: Tool executing
- `▣` Success: Tool completed successfully
- `▩` Failed: Tool returned error
- `▦` Complex: Tool with nested operations

**Worker States**:
- `⬡` Idle: Worker available, waiting for task
- `⬢` Active: Worker processing task
- `⬣` Overloaded: Worker at max capacity (throttled)

### Animation Sequences

**Task Lifecycle Animation**:
```
○ → ◔ → ◑ → ◕ → ●
```

**Spinner Animation** (existing):
```
◴ → ◵ → ◶ → ◷ → ◴ (loop)
```

**Retry Animation**:
```
◌ → ⟲ → ⏸ → ○ → ◔ → ...
```

## Visual Theme: "The Tennis Factory"

Cliffy processes tasks like a tennis ball factory:
1. **Balls (○)** enter the queue
2. **Machines (╱╲╳)** process them through stages
3. **Workers (⬢)** operate the machines
4. **Conveyor belts (→⇒⇨)** move balls between stages
5. **Quality check (▣)** verifies each operation
6. **Complete balls (●)** exit the system

This metaphor creates a cohesive visual language that's both functional and memorable.

## Color Recommendations (for future TUI)

While Cliffy is currently monochrome, future color support could use:
- **Green**: Success (●, ▣)
- **Yellow**: In-progress (◔◑◕, ⬢)
- **Red**: Error (◌, ▩, ✗)
- **Blue**: Info (⬡, ○)
- **Cyan**: Data flow (→⇒⇨)
- **Magenta**: Retry/warning (⟲, ⚠)

## Accessibility

All symbols are:
- Unicode standard characters
- Monospaced-safe (fixed width in terminals)
- Screen-reader friendly (semantic meanings)
- High contrast in monochrome
- Culturally neutral

## Next Steps

1. Implement enhanced symbols in `ascii.go`
2. Update `progress.go` to use processing states
3. Add worker visualization mode (optional flag)
4. Create data flow diagrams for debug mode
5. Test across terminal emulators
6. Gather user feedback on readability
