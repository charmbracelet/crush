# StateMachine 狀態機詳細設計

**版本**: v1.0  
**創建日期**: 2026-04-04  
**組件**: StateMachine  
**狀態**: ✅ 已完成

---

## 一、組件概述

### 1.1 功能說明

StateMachine 是事件驅動的有限狀態機，負責管理代理的生命週期狀態和狀態轉換邏輯。

### 1.2 設計目標

| 目標 | 說明 |
|------|------|
| **確定性** | 相同輸入總是產生相同輸出 |
| **可追蹤** | 記錄所有狀態轉換歷史 |
| **可觀測** | 提供狀態變化的監聽機制 |
| **可恢復** | 支援狀態持久化和恢復 |

---

## 二、狀態定義

### 2.1 代理狀態

```go
// AgentState 代理狀態
type AgentState string

const (
    StateIdle         AgentState = "idle"          // 空閒
    StateInitializing AgentState = "initializing"   // 初始化中
    StateRunning      AgentState = "running"       // 運行中
    StateWaiting      AgentState = "waiting"       // 等待中
    StateSuspended    AgentState = "suspended"     // 掛起
    StateShuttingDown AgentState = "shutting_down" // 關閉中
    StateStopped      AgentState = "stopped"       // 已停止
    StateError        AgentState = "error"         // 錯誤
)
```

### 2.2 狀態轉換圖

```
                    ┌──────────────┐
                    │  Initializing│
                    └──────┬───────┘
                           │ init_complete
                           ▼
┌──────────┐    ┌──────────────┐    ┌──────────────┐
│ Stopped  │◄───│   Idle       │───►│   Running    │
└──────────┘    └──────┬───────┘    └──────┬───────┘
     ▲                  │                   │
     │                  │ task_received    │ task_complete
     │                  ▼                   │
     │            ┌──────────────┐         │
     │            │   Waiting    │◄────────┘
     │            └──────────────┘
     │                  │
     │                  │ suspend
     │                  ▼
     │            ┌──────────────┐
     └────────────│  Suspended   │
                   └──────────────┘

     ┌──────────────┐
     │  ShuttingDown│
     └──────┬───────┘
            │ shutdown_complete
            ▼
       ┌──────────────┐
       │   Stopped    │
       └──────────────┘

     ┌──────────────┐
     │    Error    │────► (可恢復到 Idle)
     └──────────────┘
```

---

## 三、事件定義

### 3.1 事件類型

```go
// EventType 事件類型
type EventType string

const (
    EventInit           EventType = "init"             // 初始化
    EventStart          EventType = "start"            // 啟動
    EventStop           EventType = "stop"             // 停止
    EventPause          EventType = "pause"            // 暫停
    EventResume         EventType = "resume"           // 恢復
    EventTaskReceived   EventType = "task_received"    // 收到任務
    EventTaskComplete   EventType = "task_complete"    // 任務完成
    EventTaskFailed     EventType = "task_failed"      // 任務失敗
    EventHeartbeat      EventType = "heartbeat"         // 心跳
    EventError          EventType = "error"             // 錯誤
    EventTimeout        EventType = "timeout"           // 超時
    EventShutdown       EventType = "shutdown"          // 關閉
    EventSuspend        EventType = "suspend"           // 掛起
    EventResumeAction   EventType = "resume_action"     // 恢復操作
)
```

### 3.2 事件結構

```go
// Event 事件結構
type Event struct {
    ID        EventID        // 事件唯一標識
    Type      EventType      // 事件類型
    Source    AgentID        // 事件來源
    Target    AgentID        // 事件目標
    Timestamp time.Time       // 事件時間
    Payload   interface{}     // 事件數據
    Metadata  map[string]any  // 額外元數據
}
```

---

## 四、轉換規則

### 4.1 轉換規則表

| 當前狀態 | 事件 | 目標狀態 | 條件 |
|---------|------|---------|------|
| `Initializing` | `init_complete` | `Idle` | - |
| `Idle` | `task_received` | `Running` | 有可用任務 |
| `Idle` | `start` | `Running` | - |
| `Running` | `task_complete` | `Idle` | 無待處理任務 |
| `Running` | `task_complete` | `Running` | 有待處理任務 |
| `Running` | `task_received` | `Waiting` | 任務需等待資源 |
| `Running` | `error` | `Error` | 發生錯誤 |
| `Running` | `suspend` | `Suspended` | 收到掛起信號 |
| `Waiting` | `resume_action` | `Running` | 資源可用 |
| `Waiting` | `task_failed` | `Error` | 任務失敗 |
| `Suspended` | `resume` | `Running` | 收到恢復信號 |
| `Error` | `start` | `Idle` | 錯誤已清除 |
| `Idle` | `shutdown` | `ShuttingDown` | - |
| `Running` | `shutdown` | `ShuttingDown` | 當前任務完成 |
| `ShuttingDown` | `shutdown_complete` | `Stopped` | - |
| `Any` | `stop` | `Stopped` | 強制停止 |

### 4.2 轉換鉤子

```go
// TransitionHook 狀態轉換鉤子
type TransitionHook func(from, to AgentState, event Event) error

// 可註冊的鉤子
var GlobalTransitionHooks = []TransitionHook{
    logTransition,      // 記錄轉換日誌
    emitTransitionEvent, // 發布轉換事件
    updateMetrics,      // 更新指標
}
```

---

## 五、介面定義

### 5.1 StateMachine 介面

```go
// StateMachine 狀態機介面
type StateMachine interface {
    // CurrentState 獲取當前狀態
    CurrentState() AgentState
    
    // Transition 執行狀態轉換
    Transition(event Event) error
    
    // CanTransition 檢查是否可以執行轉換
    CanTransition(event Event) bool
    
    // AddTransitionRule 添加轉換規則
    AddTransitionRule(from, to AgentState, event EventType, condition TransitionCondition) error
    
    // OnStateChange 註冊狀態變化監聽器
    OnStateChange(handler StateChangeHandler)
    
    // History 獲取轉換歷史
    History() []TransitionRecord
    
    // Reset 重置狀態機
    Reset()
}
```

### 5.2 輔助介面

```go
// StateChangeHandler 狀態變化處理函數
type StateChangeHandler func(from, to AgentState, event Event)

// TransitionCondition 轉換條件函數
type TransitionCondition func(event Event) bool

// TransitionRecord 轉換記錄
type TransitionRecord struct {
    From      AgentState
    To        AgentState
    Event     EventType
    Timestamp time.Time
    Duration  time.Duration
}
```

---

## 六、實現細節

### 6.1 StateMachine 結構

```go
// StateMachine 狀態機實現
type StateMachine struct {
    mu         sync.RWMutex
    current    AgentState
    rules      map[AgentState]map[EventType][]TransitionRule
    history    []TransitionRecord
    listeners  []StateChangeHandler
    agentID    AgentID
    createdAt  time.Time
    transitions atomic.Uint64
}

type TransitionRule struct {
    To        AgentState
    Condition TransitionCondition
    Action    TransitionAction
}
```

### 6.2 核心方法

| 方法 | 說明 | 時間複雜度 |
|------|------|-----------|
| `New(agentID AgentID)` | 創建狀態機 | O(1) |
| `CurrentState()` | 獲取當前狀態 | O(1) |
| `Transition(event)` | 執行轉換 | O(1) |
| `CanTransition(event)` | 檢查可轉換性 | O(n) |
| `AddTransitionRule()` | 添加規則 | O(1) |
| `OnStateChange()` | 註冊監聽 | O(1) |
| `History()` | 獲取歷史 | O(n) |
| `Reset()` | 重置狀態機 | O(1) |

---

## 七、使用範例

### 7.1 基本使用

```go
sm := statemachine.New("agent-1")

// 註冊狀態變化監聽
sm.OnStateChange(func(from, to statemachine.AgentState, event statemachine.Event) {
    fmt.Printf("Agent %s: %s -> %s (event: %s)\n", 
        "agent-1", from, to, event.Type)
})

// 執行轉換
err := sm.Transition(statemachine.Event{
    Type:   statemachine.EventStart,
    Source: "agent-1",
})
```

### 7.2 自定義轉換規則

```go
// 添加帶條件的轉換規則
sm.AddTransitionRule(
    statemachine.StateRunning, 
    statemachine.StateError,
    statemachine.EventError,
    func(e statemachine.Event) bool {
        // 只有嚴重錯誤才轉換到 Error 狀態
        if err, ok := e.Payload.(error); ok {
            return isSevereError(err)
        }
        return false
    },
)
```

---

## 八、線程安全性

### 8.1 鎖策略

| 字段 | 鎖類型 | 說明 |
|------|--------|------|
| `current` | `sync.RWMutex` | 保護當前狀態 |
| `rules` | `sync.RWMutex` | 保護轉換規則 |
| `history` | `sync.RWMutex` | 保護歷史記錄 |
| `listeners` | `sync.RWMutex` | 保護監聽器列表 |
| `transitions` | `atomic.Uint64` | 原子計數器 |

### 8.2 並發約束

```
1. 所有公共方法都是線程安全的
2. 監聽器回調在單獨的 goroutine 中執行
3. 歷史記錄有最大長度限制 (預設 1000 條)
```

---

## 九、測試策略

### 9.1 狀態轉換測試

```go
func TestStateMachine_BasicTransitions(t *testing.T) {
    sm := New("test-agent")
    
    // 測試初始狀態
    assert.Equal(t, StateInitializing, sm.CurrentState())
    
    // 測試初始化完成
    err := sm.Transition(Event{Type: EventInit})
    assert.NoError(t, err)
    assert.Equal(t, StateIdle, sm.CurrentState())
    
    // 測試啟動
    err = sm.Transition(Event{Type: EventStart})
    assert.NoError(t, err)
    assert.Equal(t, StateRunning, sm.CurrentState())
}
```

### 9.2 無效轉換測試

```go
func TestStateMachine_InvalidTransition(t *testing.T) {
    sm := New("test-agent")
    
    // 嘗試從 Initializing 直接到 Running (無效)
    err := sm.Transition(Event{Type: EventStart})
    assert.Error(t, err) // 應該失敗
    
    assert.Equal(t, StateInitializing, sm.CurrentState()) // 狀態不變
}
```

### 9.3 並發測試

```go
func TestStateMachine_ConcurrentTransitions(t *testing.T) {
    sm := New("test-agent")
    _ = sm.Transition(Event{Type: EventInit}) // 先初始化
    
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            sm.Transition(Event{Type: EventStart})
        }()
    }
    wg.Wait()
    
    // 驗證最終狀態一致
    assert.True(t, sm.transitions.Load() > 0)
}
```

---

## 十、性能考量

### 10.1 歷史記錄限制

```go
const MaxHistorySize = 1000 // 最大歷史記錄數

func (sm *StateMachine) addHistory(record TransitionRecord) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    sm.history = append(sm.history, record)
    if len(sm.history) > MaxHistorySize {
        sm.history = sm.history[len(sm.history)-MaxHistorySize:]
    }
}
```

### 10.2 監聽器限制

```go
const MaxListeners = 10 // 最大監聽器數
```

---

## 十一、檔案位置

```
internal/agent/statmachine/
└── state_machine.go          # 主實現 (~280 行)
```

---

## 十二、依賴關係

```
依賴:
    └─ 無 (完全獨立)

被依賴:
    ├─ SwarmExt (狀態同步)
    ├─ Guardian (狀態監控)
    └─ Coordinator (任務調度)
```

---

## 十三、擴展方向

| 擴展項 | 說明 |
|--------|------|
| **層次狀態機** | 支援嵌套狀態 |
| **並行狀態機** | 多個狀態並行 |
| **狀態持久化** | 保存/恢復狀態 |
| **狀態預測** | 預測未來狀態 |

---

*文檔更新日期: 2026-04-04*
