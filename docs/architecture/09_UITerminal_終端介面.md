# UITerminal 終端介面

## 概述

UITerminal 是 CrushCL 多代理系統中的終端使用者介面元件，負責提供代理系統的可視化監控、任務管理和互動操作介面。

## 設計目標

| 目標 | 說明 |
|------|------|
| **即時監控** | 實時顯示代理狀態和任務進度 |
| **操作介面** | 提供任務提交和管理介面 |
| **日誌查看** | 集中的日誌查看和搜索 |
| **效能監控** | 系統資源和使用率監控 |
| **命令列整合** | 與現有終端命令整合 |

## 終端 UI 佈局

```
┌─────────────────────────────────────────────────────────────┐
│  CrushCL Multi-Agent Control Center                         │
├─────────────────────────────────────────────────────────────┤
│  [Agents: 5]  [Tasks: 12]  [Running: 3]  [Memory: 67%]    │
├───────────────────────┬─────────────────────────────────────┤
│  AGENTS               │  TASK MONITOR                      │
│  ─────────────        │  ─────────────                      │
│  ● agent-1  [RUN]    │  task-101  ████████░░ 80%         │
│  ● agent-2  [RUN]    │  task-102  ████░░░░░░ 40%         │
│  ○ agent-3  [IDLE]   │  task-103  ░░░░░░░░░░ 0%          │
│  ○ agent-4  [IDLE]    │  task-104  ██████████ 100%        │
│  ● agent-5  [ERR]     │                                    │
├───────────────────────┴─────────────────────────────────────┤
│  SYSTEM STATUS                                             │
│  ───────────────                                           │
│  CPU: ████████░░ 78%  │  Memory: ██████░░░░ 62%           │
│  Disk: ███░░░░░░░ 34% │  Network: ██████████ 95%          │
├─────────────────────────────────────────────────────────────┤
│  > _                                                          │
└─────────────────────────────────────────────────────────────┘
```

## 核心類型

### UITerminal 結構

```go
type UITerminal struct {
    mu          sync.RWMutex
    config      TerminalConfig
    renderer    *TerminalRenderer
    input       *InputHandler
    output      *OutputBuffer
    components  map[string]UIComponent
    layout      *LayoutManager
    eventLoop   *EventLoop
    bindings    *KeyBindings
}

type TerminalConfig struct {
    Width           int
    Height          int
    RefreshRate     time.Duration    // 刷新率
    Theme           TerminalTheme
    ShowBorder      bool
    ShowStatusBar   bool
    ShowHelpBar     bool
    Scrollback      int             // 滾動緩存行數
    MaxAgentsDisplay int            // 最大顯示代理數
    MaxTasksDisplay int            // 最大顯示任務數
}

type TerminalTheme struct {
    Primary       string
    Secondary     string
    Success       string
    Warning       string
    Error         string
    Background    string
    Foreground    string
    Border        string
    AgentRunning  string
    AgentIdle     string
    AgentError    string
}
```

## UI 元件

### 1. 狀態欄元件

```go
type StatusBar struct {
    width     int
    sections  []StatusSection
    listeners []StatusListener
}

type StatusSection struct {
    Label   string
    Value   string
    Color   string
    Width   int
}

type StatusListener interface {
    OnStatusUpdate(section string, value string)
}

func (sb *StatusBar) Render() string {
    sb.mu.RLock()
    defer sb.mu.RUnlock()
    
    var sb strings.Builder
    sb.WriteString("┌─")
    
    for i, section := range sb.sections {
        if i > 0 {
            sb.WriteString("─┬─")
        }
        
        label := fmt.Sprintf("%s:", section.Label)
        value := section.Value
        content := fmt.Sprintf("%s %s", label, value)
        
        // 確保寬度一致
        padded := sb.pad(content, section.Width)
        sb.WriteString(padded)
    }
    
    sb.WriteString("─┐")
    return sb.String()
}
```

### 2. 代理列表元件

```go
type AgentList struct {
    mu       sync.RWMutex
    agents   map[string]*AgentStatus
    selected string
    config   ListConfig
}

type AgentStatus struct {
    ID        string
    Name      string
    State     AgentState
    CPU       float64
    Memory    float64
    Tasks     int
    LastSeen  time.Time
}

type AgentState int

const (
    AgentStateRunning AgentState = iota
    AgentStateIdle
    AgentStateError
    AgentStateOffline
)

func (al *AgentList) Render() string {
    al.mu.RLock()
    defer al.mu.RUnlock()
    
    var sb strings.Builder
    
    // 標題
    sb.WriteString("  AGENTS")
    sb.WriteString(strings.Repeat("─", 50))
    sb.WriteString("\n")
    
    // 排序：running > idle > error > offline
    sorted := al.sortByState()
    
    for _, agent := range sorted {
        indicator := al.getStateIndicator(agent.State)
        stateStr := al.getStateString(agent.State)
        
        line := fmt.Sprintf("  %s %s [%s] CPU:%.1f%% MEM:%.1f%%",
            indicator,
            agent.Name,
            stateStr,
            agent.CPU,
            agent.Memory,
        )
        
        // 選中效果
        if agent.ID == al.selected {
            line = "> " + line
        } else {
            line = "  " + line
        }
        
        sb.WriteString(line)
        sb.WriteString("\n")
    }
    
    return sb.String()
}

func (al *AgentList) getStateIndicator(state AgentState) string {
    switch state {
    case AgentStateRunning:
        return "●" // 綠色
    case AgentStateIdle:
        return "○" // 灰色
    case AgentStateError:
        return "✗" // 紅色
    case AgentStateOffline:
        return "○" // 灰色
    default:
        return "?"
    }
}
```

### 3. 任務監控元件

```go
type TaskMonitor struct {
    mu    sync.RWMutex
    tasks map[string]*TaskProgress
    config MonitorConfig
}

type TaskProgress struct {
    ID          string
    Name        string
    Status      TaskStatus
    Progress    float64     // 0.0 - 1.0
    AgentID     string
    StartTime   time.Time
    ETA         time.Duration
    Logs        []string
}

type TaskStatus int

const (
    TaskStatusPending TaskStatus = iota
    TaskStatusRunning
    TaskStatusCompleted
    TaskStatusFailed
    TaskStatusCancelled
)

func (tm *TaskMonitor) Render() string {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    var sb strings.Builder
    
    // 標題
    sb.WriteString("  TASK MONITOR")
    sb.WriteString(strings.Repeat("─", 44))
    sb.WriteString("\n")
    
    // 按進度排序
    sorted := tm.sortByProgress()
    
    for _, task := range sorted {
        // 進度條
        bar := tm.renderProgressBar(task.Progress)
        
        // 時間
        elapsed := time.Since(task.StartTime)
        timeStr := fmt.Sprintf("%v", elapsed.Truncate(time.Second))
        
        line := fmt.Sprintf("  %s %s %s %s",
            task.ID[:8],
            bar,
            fmt.Sprintf("%.0f%%", task.Progress*100),
            timeStr,
        )
        
        sb.WriteString(line)
        sb.WriteString("\n")
    }
    
    return sb.String()
}

func (tm *TaskMonitor) renderProgressBar(progress float64) string {
    width := 20
    filled := int(progress * float64(width))
    empty := width - filled
    
    bar := "█" * filled + "░" * empty
    return "[" + bar + "]"
}
```

### 4. 系統監控元件

```go
type SystemMonitor struct {
    mu       sync.RWMutex
    metrics  SystemMetrics
    history  []SystemMetrics
    maxHistory int
}

type SystemMetrics struct {
    CPU        float64
    Memory     float64
    Disk       float64
    Network    float64
    Timestamp  time.Time
}

func (sm *SystemMonitor) Render() string {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    var sb strings.Builder
    
    sb.WriteString("  SYSTEM STATUS")
    sb.WriteString(strings.Repeat("─", 45))
    sb.WriteString("\n")
    
    // CPU
    cpuBar := sm.renderMeter(sm.metrics.CPU)
    sb.WriteString(fmt.Sprintf("  CPU:    %s %.0f%%\n", cpuBar, sm.metrics.CPU))
    
    // Memory
    memBar := sm.renderMeter(sm.metrics.Memory)
    sb.WriteString(fmt.Sprintf("  Memory: %s %.0f%%\n", memBar, sm.metrics.Memory))
    
    // Disk
    diskBar := sm.renderMeter(sm.metrics.Disk)
    sb.WriteString(fmt.Sprintf("  Disk:   %s %.0f%%\n", diskBar, sm.metrics.Disk))
    
    // Network
    netBar := sm.renderMeter(sm.metrics.Network)
    sb.WriteString(fmt.Sprintf("  Network:%s %.0f%%\n", netBar, sm.metrics.Network))
    
    return sb.String()
}

func (sm *SystemMonitor) renderMeter(percent float64) string {
    width := 10
    filled := int(percent / 100 * float64(width))
    empty := width - filled
    
    // 顏色：< 60% 綠色，60-80% 黃色，> 80% 紅色
    color := green
    if percent >= 60 && percent < 80 {
        color = yellow
    } else if percent >= 80 {
        color = red
    }
    
    bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
    return color + bar + reset
}
```

### 5. 命令輸入元件

```go
type CommandInput struct {
    mu           sync.RWMutex
    prompt       string
    history      []string
    historyIndex int
    currentInput string
    completers   []Completer
}

type Completer interface {
    Complete(input string) []string
}

func (ci *CommandInput) HandleInput(key Key) string {
    ci.mu.Lock()
    defer ci.mu.Unlock()
    
    switch key {
    case KeyEnter:
        cmd := ci.currentInput
        if cmd != "" {
            ci.history = append(ci.history, cmd)
        }
        ci.historyIndex = len(ci.history)
        ci.currentInput = ""
        return cmd
        
    case KeyBackspace:
        if len(ci.currentInput) > 0 {
            ci.currentInput = ci.currentInput[:len(ci.currentInput)-1]
        }
        
    case KeyUp:
        if ci.historyIndex > 0 {
            ci.historyIndex--
            ci.currentInput = ci.history[ci.historyIndex]
        }
        
    case KeyDown:
        if ci.historyIndex < len(ci.history)-1 {
            ci.historyIndex++
            ci.currentInput = ci.history[ci.historyIndex]
        } else {
            ci.historyIndex = len(ci.history)
            ci.currentInput = ""
        }
        
    case KeyTab:
        // 自動補全
        completions := ci.getCompletions(ci.currentInput)
        if len(completions) == 1 {
            ci.currentInput = completions[0]
        }
        
    default:
        ci.currentInput += string(key)
    }
    
    return ""
}

func (ci *CommandInput) Render() string {
    ci.mu.RLock()
    defer ci.mu.RUnlock()
    
    return fmt.Sprintf("  > %s_", ci.currentInput)
}
```

## 佈局管理

```go
type LayoutManager struct {
    mu       sync.RWMutex
    config   LayoutConfig
    panels   map[string]*Panel
    grid     *GridLayout
}

type Panel struct {
    ID       string
    Name     string
    Component UIComponent
    X, Y     int
    Width, Height int
    Visible  bool
    Border   bool
}

type GridLayout struct {
    Rows    int
    Cols    int
    Cells    [][]*Panel
}

func (lm *LayoutManager) Render() string {
    lm.mu.RLock()
    defer lm.mu.RUnlock()
    
    var sb strings.Builder
    
    // 頂部邊框
    sb.WriteString("┌")
    sb.WriteString(strings.Repeat("─", lm.config.Width-2))
    sb.WriteString("┐\n")
    
    // 渲染每個可見面板
    for _, panel := range lm.panels {
        if !panel.Visible {
            continue
        }
        
        if panel.Border {
            sb.WriteString("│")
        }
        
        content := panel.Component.Render()
        sb.WriteString(content)
        
        if panel.Border {
            sb.WriteString("│")
        }
        sb.WriteString("\n")
    }
    
    // 底部邊框
    sb.WriteString("└")
    sb.WriteString(strings.Repeat("─", lm.config.Width-2))
    sb.WriteString("┘")
    
    return sb.String()
}
```

## 事件處理

```go
type EventLoop struct {
    mu       sync.RWMutex
    terminal *UITerminal
    running  bool
    events   chan Event
    handlers map[EventType][]EventHandler
}

type Event struct {
    Type    EventType
    Payload interface{}
    Source  string
    Time    time.Time
}

type EventType int

const (
    EventTypeKey EventType = iota
    EventTypeResize
    EventTypeAgentUpdate
    EventTypeTaskUpdate
    EventTypeSystemMetrics
    EventTypeCommand
)

type EventHandler func(Event) error

func (el *EventLoop) Run() error {
    el.mu.Lock()
    el.running = true
    el.mu.Unlock()
    
    for el.running {
        select {
        case event := <-el.events:
            if err := el.dispatch(event); err != nil {
                return err
            }
        case <-time.After(el.terminal.config.RefreshRate):
            el.terminal.refresh()
        }
    }
    
    return nil
}

func (el *EventLoop) dispatch(event Event) error {
    handlers := el.handlers[event.Type]
    
    for _, handler := range handlers {
        if err := handler(event); err != nil {
            return err
        }
    }
    
    return nil
}
```

## 快捷鍵綁定

```go
type KeyBindings struct {
    mu       sync.RWMutex
    bindings map[Key]*Binding
}

type Binding struct {
    Key       Key
    Modifiers []KeyModifier
    Command   string
    Handler   func() error
    Description string
}

func (kb *KeyBindings) HandleKey(key Key, modifiers []KeyModifier) error {
    kb.mu.RLock()
    defer kb.mu.RUnlock()
    
    binding := kb.findBinding(key, modifiers)
    if binding == nil {
        return nil
    }
    
    if binding.Handler != nil {
        return binding.Handler()
    }
    
    return nil
}

// 預設快捷鍵
var defaultBindings = []*Binding{
    {Key: KeyCtrlC, Command: "cancel", Description: "Cancel current operation"},
    {Key: KeyCtrlZ, Command: "suspend", Description: "Suspend current task"},
    {Key: KeyCtrlL, Command: "clear", Description: "Clear screen"},
    {Key: KeyCtrlR, Command: "search", Description: "Search history"},
    {Key: KeyTab, Command: "complete", Description: "Auto-complete"},
    {Key: KeyUp, Command: "history-up", Description: "Previous command"},
    {Key: KeyDown, Command: "history-down", Description: "Next command"},
    {Key: KeyCtrlA, Command: "select-agent-1", Description: "Select agent 1"},
    {Key: KeyCtrlB, Command: "select-agent-2", Description: "Select agent 2"},
    {Key: KeyF1, Command: "help", Description: "Show help"},
    {Key: KeyF2, Command: "agents", Description: "Show agent list"},
    {Key: KeyF3, Command: "tasks", Description: "Show task list"},
    {Key: KeyF4, Command: "system", Description: "Show system status"},
    {Key: KeyF5, Command: "refresh", Description: "Refresh display"},
}
```

## 使用範例

### 基本用法

```go
// 創建終端介面
terminal := NewUITerminal(TerminalConfig{
    Width:           120,
    Height:          40,
    RefreshRate:     250 * time.Millisecond,
    Scrollback:      1000,
    MaxAgentsDisplay: 20,
    MaxTasksDisplay: 50,
})

// 添加面板
terminal.AddPanel(&Panel{
    ID:       "agents",
    Name:     "Agents",
    Component: NewAgentList(),
    X:        0, Y: 0,
    Width:    40, Height: 30,
    Border:   true,
})

terminal.AddPanel(&Panel{
    ID:       "tasks",
    Name:     "Tasks",
    Component: NewTaskMonitor(),
    X:        40, Y: 0,
    Width:    80, Height: 30,
    Border:   true,
})

// 啟動事件循環
go terminal.Run()

// 處理命令
terminal.HandleCommand("list agents")
terminal.HandleCommand("status task-123")
terminal.HandleCommand("cancel task-456")
```

### 與代理系統整合

```go
type AgentSystemWithUI struct {
    agents   *SwarmExt
    terminal *UITerminal
}

func (as *AgentSystemWithUI) Start() {
    // 啟動代理系統
    as.agents.Start()
    
    // 設置 UI 回調
    as.terminal.AddEventHandler(EventTypeAgentUpdate, func(e Event) error {
        update := e.Payload.(*AgentUpdate)
        as.terminal.UpdatePanel("agents", &AgentStatus{
            ID:     update.AgentID,
            Name:   update.Name,
            State:  update.State,
            CPU:    update.CPU,
            Memory: update.Memory,
        })
        return nil
    })
    
    as.terminal.AddEventHandler(EventTypeTaskUpdate, func(e Event) error {
        update := e.Payload.(*TaskUpdate)
        as.terminal.UpdatePanel("tasks", &TaskProgress{
            ID:       update.TaskID,
            Progress: update.Progress,
            Status:   update.Status,
        })
        return nil
    })
    
    // 啟動 UI
    go as.terminal.Run()
}
```

## 命令列整合

```go
type TerminalCommands struct {
    terminal *UITerminal
    agentSystem *AgentSystem
}

func (tc *TerminalCommands) RegisterCommands() {
    tc.terminal.RegisterCommand("list", tc.listAgents)
    tc.terminal.RegisterCommand("status", tc.showStatus)
    tc.terminal.RegisterCommand("cancel", tc.cancelTask)
    tc.terminal.RegisterCommand("submit", tc.submitTask)
    tc.terminal.RegisterCommand("pause", tc.pauseAgent)
    tc.terminal.RegisterCommand("resume", tc.resumeAgent)
    tc.terminal.RegisterCommand("logs", tc.showLogs)
}

func (tc *TerminalCommands) listAgents(args []string) error {
    agents, _ := tc.agentSystem.ListAgents()
    
    for _, agent := range agents {
        fmt.Printf("%s [%s] CPU:%.1f%% MEM:%.1f%%\n",
            agent.Name, agent.State, agent.CPU, agent.Memory)
    }
    
    return nil
}
```

## 錯誤處理

```go
var (
    ErrTerminalNotSupported = errors.New("terminal not supported")
    ErrInvalidPanelSize    = errors.New("invalid panel size")
    ErrPanelNotFound       = errors.New("panel not found")
    ErrCommandFailed       = errors.New("command execution failed")
)
```

## 與其他元件的整合

| 元件 | 整合方式 |
|------|---------|
| SwarmExt | 顯示代理和任務狀態 |
| Guardian | 顯示錯誤和告警 |
| StateMachine | 顯示狀態變化 |
| MessageBus | 接收狀態更新事件 |

## 下一步

- [ ] 實現完整的終端渲染引擎
- [ ] 添加滑鼠支援
- [ ] 實現分頁和滾動
- [ ] 添加主題支援
- [ ] 實現遠端終端模式
