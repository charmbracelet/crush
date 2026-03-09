# Orchestration Visualization Design

## Overview

A Linear-inspired visualization layer for observing multi-agent orchestration. Users can oversee work being done, see communication between agents, and track overall progress.

---

## Design Philosophy

**Like Linear, but for AI Agents:**
- Clean, minimal interface
- Real-time updates without refresh
- Status badges and progress indicators
- Expandable cards for details
- Activity timeline for communication flow
- Keyboard-first navigation

---

## Core Views

### 1. Orchestra Board (Main View)

The primary view showing all agents and their current state.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 🎼 Orchestra                        🔍 Filter...    ⚙ Settings  □ Close│
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  TASK: Implement REST API for user authentication                       │
│  Progress: ████████░░░░░░░░░░░░ 40%  •  Turn 12/50  •  Est. 3 min left │
│                                                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │ 🧠 PLANNER  │  │ 💻 CODER    │  │ 🔍 REVIEWER │  │ 🧪 TESTER   │    │
│  │─────────────│  │─────────────│  │─────────────│  │─────────────│    │
│  │ Status:     │  │ Status:     │  │ Status:     │  │ Status:     │    │
│  │ ⏸ WAITING  │  │ ⚡ ACTIVE   │  │ ✓ IDLE     │  │ ✓ IDLE     │    │
│  │             │  │             │  │             │  │             │    │
│  │ Model:      │  │ Model:      │  │ Model:      │  │ Model:      │    │
│  │ glm-4-plus  │  │ claude-3.5  │  │ o3-mini    │  │ glm-4       │    │
│  │             │  │             │  │             │  │             │    │
│  │ Current:    │  │ Current:    │  │ Completed:  │  │ Waiting:    │    │
│  │ Waiting for │  │ Writing     │  │ 3 reviews   │  │ code to be  │    │
│  │ coder       │  │ auth.go     │  │             │  │ ready       │    │
│  │             │  │ ████░░ 60% │  │             │  │             │    │
│  │ Turns: 4    │  │ Turns: 8    │  │ Turns: 3    │  │ Turns: 2    │    │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘    │
│                                                                          │
│  Legend: ⚡ Active  ⏸ Waiting  ✓ Idle  ❌ Error  ⏳ Starting           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2. Activity Timeline (Communication View)

Shows real-time communication between agents in a Linear-style activity feed.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 📋 Activity Timeline                                    [All ▼] [Live ●]│
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Now                                                                     │
│  ├── 💻 CODER → 🔍 REVIEWER                                             │
│ │   "Completed auth.go implementation, ready for review"                │
│ │   📎 3 files changed, +247 -12                                        │
│ │                                                                        │
│  2m ago                                                                  │
│  ├── 🧠 PLANNER                                                         │
│ │   "Updated plan: added edge case handling for token refresh"         │
│ │                                                                        │
│  5m ago                                                                  │
│  ├── 💻 CODER                                                           │
│ │   "Starting implementation of JWT validation..."                      │
│ │   ████░░░░░░░░░░░░░░░░ 20%                                            │
│ │                                                                        │
│  7m ago                                                                  │
│  ├── 🧠 PLANNER → 💻 CODER                                              │
│ │   "Task assigned: Implement /auth/login endpoint"                     │
│ │                                                                        │
│  10m ago                                                                 │
│  ├── 🎯 USER                                                            │
│ │   "Implement REST API for user authentication"                        │
│ │                                                                        │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3. Task Board (Kanban View)

Linear-style kanban showing task decomposition and assignment.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 📋 Task Board                                          [+ Add Task]     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  BACKLOG          IN PROGRESS           IN REVIEW           DONE        │
│  ────────         ───────────           ─────────           ────        │
│                                                                          │
│  ┌──────────┐     ┌──────────────┐     ┌──────────────┐     ┌─────────┐│
│  │ Add rate │     │ Implement    │     │ Review auth  │     │ Setup   ││
│  │ limiting │     │ /auth/login  │     │ middleware   │     │ project ││
│  │          │     │              │     │              │     │         ││
│  │ P3       │     │ 💻 CODER     │     │ 🔍 REVIEWER  │     │ ✓       ││
│  │ No agent │     │ ████████░░   │     │ Waiting      │     │ 5m      ││
│  └──────────┘     └──────────────┘     └──────────────┘     └─────────┘│
│                                                                          │
│  ┌──────────┐     ┌──────────────┐                           ┌─────────┐│
│  │ Add 2FA  │     │ Write tests  │                           │ Design  ││
│  │ support  │     │              │                           │ schema  ││
│  │          │     │ 🧪 TESTER    │                           │         ││
│  │ P2       │     │ █░░░░░░░░░░  │                           │ ✓       ││
│  │ No agent │     │ 10%          │                           │ 12m     ││
│  └──────────┘     └──────────────┘                           └─────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 4. Agent Detail Panel (Expanded View)

Click an agent card to see detailed view with live output.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 💻 CODER                                           [Minimize] [Detach] │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Status: ⚡ ACTIVE    Model: claude-3.5-sonnet    CLI: crush            │
│  Current Task: Implement /auth/login endpoint                           │
│                                                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Live Output:                                                  [⬇ End] │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │ > Analyzing existing codebase structure...                        │  │
│  │ > Found: go.mod, main.go, internal/                               │  │
│  │ > Creating internal/auth/jwt.go                                   │  │
│  │ > Writing ValidateToken function...                               │  │
│  │ ✓ File created: internal/auth/jwt.go                              │  │
│  │ > Creating internal/handlers/auth.go                              │  │
│  │ > Writing LoginHandler...                                         │  │
│  │ █ Writing...                                                      │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  Tools Used: file_read(3), file_write(2), execute(1)                    │
│  Tokens: 4,521 prompt + 1,234 completion = 5,755 total                  │
│                                                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Handoffs Available: [🔍 reviewer] [🧪 tester] [🧠 planner]             │
│                                                                          │
│  [↵ Handoff to...] [⏸ Pause] [🔄 Restart] [📋 View History]            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5. Ledger View (Task State)

Shows the task and progress ledgers - the "brain" of orchestration.

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 📒 Ledger                                                     [Refresh] │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─ TASK LEDGER ─────────────────────────────────────────────────────┐  │
│  │                                                                     │  │
│  │  Original Task:                                                     │  │
│  │  "Implement REST API for user authentication"                       │  │
│  │                                                                     │  │
│  │  Facts Discovered:                                                  │  │
│  │  • Project uses Go 1.21 with Gin framework                          │  │
│  │  • PostgreSQL database already configured                           │  │
│  │  • JWT library: github.com/golang-jwt/jwt/v5                       │  │
│  │  • No existing auth middleware                                      │  │
│  │                                                                     │  │
│  │  Current Plan:                                                      │  │
│  │  1. ✓ Setup project structure (planner)                             │  │
│  │  2. ✓ Design user schema (planner)                                  │  │
│  │  3. ⚡ Implement auth endpoints (coder)                              │  │
│  │  4. ⏳ Add JWT middleware (coder)                                    │  │
│  │  5. ⏳ Review implementation (reviewer)                              │  │
│  │  6. ⏳ Write tests (tester)                                          │  │
│  │                                                                     │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌─ PROGRESS LEDGER ─────────────────────────────────────────────────┐  │
│  │                                                                     │  │
│  │  Turn: 12/50    Stalls: 1/3    Last Agent: coder                   │  │
│  │                                                                     │  │
│  │  Recent Messages:                                                   │  │
│  │  [12] coder: "Completed login endpoint, starting token refresh"    │  │
│  │  [11] planner: "Updated fact: JWT expires in 24h"                  │  │
│  │  [10] coder: "Implementing /auth/login..."                         │  │
│  │                                                                     │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Visual Components

### Agent Status Indicators

```
⚡ ACTIVE     - Currently processing (animated pulse)
⏸ WAITING    - Blocked, waiting for another agent
✓ IDLE       - Available for work
❌ ERROR      - Failed, needs attention
⏳ STARTING   - Spawning subprocess
🔄 HANDOFF    - Transitioning to another agent
```

### Progress Indicators

```
████████░░░░░░░░░░░░ 40%   - Task progress
▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 100%  - Complete
░░░░░░░░░░░░░░░░░░░░ 0%    - Not started
```

### Connection Lines (Handoffs)

```
┌─────────┐                    ┌─────────┐
│ PLANNER │ ─ ─ ─ handoff ─ ─▶ │ CODER   │
└─────────┘                    └─────────┘

┌─────────┐                    ┌─────────┐
│ CODER   │ ─ ─ parallel ─ ─ ─▶│ TESTER  │
└─────────┘                    └─────────┘
     │
     └────────────────────────▶│ REVIEWER│
                                └─────────┘
```

### Token Usage Bars

```
Tokens: ████████░░░░░░░░░░░░ 4,521 / 10,000 (45%)
Cost:   $0.12 this turn, $1.34 total
```

---

## Interaction Patterns

### Keyboard Navigation

```
j/k or ↑/↓     - Navigate between agents
Enter          - Expand agent detail
h/l or ←/→     - Switch views (Board/Timeline/Kanban/Ledger)
/              - Focus search/filter
r              - Refresh current view
p              - Pause/resume orchestration
?              - Show help
q              - Close view / Exit
```

### Mouse Interactions

```
Click agent card    - Expand detail panel
Click task card     - Show task details
Click handoff line  - Show handoff message
Scroll timeline     - View history
Hover status        - Show tooltip with details
```

---

## Real-Time Updates

### State Changes (pushed to UI immediately)

```go
type UIEvent struct {
    Type      UIEventType `json:"type"`
    Timestamp time.Time   `json:"timestamp"`
    Agent     string      `json:"agent,omitempty"`
    Data      any         `json:"data,omitempty"`
}

type UIEventType string

const (
    EventAgentStatus    UIEventType = "agent_status"     // Status changed
    EventAgentOutput    UIEventType = "agent_output"     // New output line
    EventAgentHandoff   UIEventType = "agent_handoff"    // Handoff occurred
    EventTaskUpdate     UIEventType = "task_update"      // Task progress
    EventLedgerUpdate   UIEventType = "ledger_update"    // Facts/plan changed
    EventOrchestration  UIEventType = "orchestration"    // Turn change
    EventError          UIEventType = "error"            // Error occurred
    EventComplete       UIEventType = "complete"         // Task complete
)
```

### Update Channels

```go
type OrchestraUI struct {
    events     chan UIEvent
    agents     map[string]*AgentCard
    timeline   *Timeline
    board      *TaskBoard
    ledger     *LedgerView
    activeView ViewType
}

func (ui *OrchestraUI) handleEvent(event UIEvent) {
    switch event.Type {
    case EventAgentStatus:
        ui.agents[event.Agent].SetStatus(event.Data.(AgentStatus))
    case EventAgentOutput:
        ui.agents[event.Agent].AppendOutput(event.Data.(string))
    case EventAgentHandoff:
        ui.timeline.AddHandoff(event.Data.(HandoffEvent))
    // ...
    }
}
```

---

## UI Component Types (Go)

```go
// Agent card in the board view
type AgentCard struct {
    Name        string
    Role        string
    Status      AgentStatus
    Model       string
    CurrentTask string
    Progress    float64
    Turns       int
    Output      *OutputStream
    Selected    bool
}

// Timeline entry
type TimelineEntry struct {
    Time      time.Time
    AgentFrom string // empty for user/system
    AgentTo   string // empty for broadcast
    Content   string
    Metadata  map[string]any // files changed, tokens, etc.
}

// Task card in kanban
type TaskCard struct {
    ID          string
    Title       string
    Description string
    Status      TaskStatus // backlog, in_progress, in_review, done
    Assignee    string     // agent name
    Priority    int
    Progress    float64
}

// Ledger view model
type LedgerViewModel struct {
    TaskLedger     *TaskLedger
    ProgressLedger *ProgressLedger
    Expanded       bool
}
```

---

## Layout Modes

### Split View (Default)

```
┌──────────────────────────────────────────────────────────┐
│ Board View (agents)           │ Timeline (activity)      │
│                               │                          │
│  ┌────┐ ┌────┐ ┌────┐ ┌────┐ │  Now                     │
│  │    │ │    │ │    │ │    │ │  ├── event               │
│  └────┘ └────┘ └────┘ └────┘ │  ├── event               │
│                               │  └── event               │
│  Progress: ████░░░░ 40%       │                          │
└──────────────────────────────────────────────────────────┘
```

### Focus View (Agent Detail)

```
┌──────────────────────────────────────────────────────────┐
│ Agent: CODER                                              │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  [Full agent detail takes entire view]                   │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### Minimized (Overlay)

```
┌──────────────────────────────────────────────────────────┐
│ [Main Chat View]                                          │
│                                                          │
│                                          ┌──────────────┐│
│                                          │ 🎼 3 agents  ││
│                                          │ ⚡coder 40%  ││
│                                          │ [Expand]     ││
│                                          └──────────────┘│
└──────────────────────────────────────────────────────────┘
```

---

## Animations

### Status Transitions

```go
// Pulse animation for active agents
func (c *AgentCard) RenderActive() string {
    pulse := c.animation.Frame() // 0.0 - 1.0
    glow := lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("#00ff00").Alpha(pulse))
    return glow.Render(c.content)
}

// Progress bar animation
func progressBar(progress float64, width int) string {
    filled := int(progress * float64(width))
    bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
    return lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render(bar)
}
```

### Handoff Animation

```
PLANNER ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ▶ CODER
         ^ animated dot traveling
```

---

## Integration with Crush TUI

### View Switching

```go
// In main UI model
type ViewMode int

const (
    ViewChat ViewMode = iota
    ViewOrchestra
    ViewOrchestraOverlay
)

func (m *Model) HandleKey(key.Key) {
    switch key {
    case "ctrl+o":
        m.toggleOrchestraView()
    case "ctrl+shift+o":
        m.toggleOrchestraOverlay()
    }
}
```

### Orchestra as Overlay

```go
type OrchestraOverlay struct {
    common     *common.Common
    orchestra  *OrchestraUI
    minimized  bool
    position   OverlayPosition // bottom-right, right-pane, full
    width      int
    height     int
}

func (o *OrchestraOverlay) Render() string {
    if o.minimized {
        return o.renderMinimized()
    }
    return o.orchestra.Render(o.width, o.height)
}
```

---

## Configuration

```yaml
# Visualization settings
ui:
  orchestra:
    enabled: true
    default_view: "split"      # split, board, timeline, kanban, ledger
    overlay_position: "right"  # right, bottom, overlay
    update_interval: 100ms     # UI refresh rate
    animations: true
    show_tokens: true
    show_costs: true

  agent_cards:
    show_model: true
    show_turns: true
    show_progress: true

  timeline:
    max_entries: 100
    show_metadata: true

  kanban:
    columns: [backlog, in_progress, in_review, done]
```

---

## Implementation Priority

**Phase 1: Core Visualization**
- [ ] AgentCard component
- [ ] Basic board layout
- [ ] Status updates via events
- [ ] Timeline component

**Phase 2: Interactivity**
- [ ] Keyboard navigation
- [ ] Click to expand
- [ ] View switching
- [ ] Search/filter

**Phase 3: Advanced Features**
- [ ] Kanban view with task cards
- [ ] Ledger view
- [ ] Handoff animations
- [ ] Cost/tracking displays

**Phase 4: Polish**
- [ ] Smooth animations
- [ ] Responsive layout
- [ ] Export views
- [ ] Screenshot/share
