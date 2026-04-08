# Guardian 守護者系統詳細設計

**版本**: v1.0  
**創建日期**: 2026-04-04  
**組件**: Guardian / GuardianExt  
**狀態**: ✅ 已完成

---

## 一、組件概述

### 1.1 功能說明

Guardian 是 CrushCL 多代理系統的故障檢測和保護機制，提供心跳監控和熔斷保護功能。原始 Guardian 為嵌入式設計，GuardianExt 將其拆分為獨立的 HeartbeatAgent 和 CircuitBreakerAgent。

### 1.2 組件變體

| 變體 | 檔案 | 說明 |
|------|------|------|
| **Guardian** | `guardian.go` | 原始嵌入式設計，心跳和熔斷在一個結構中 |
| **GuardianExt** | `guardian_ext.go` | 擴展設計，分離為獨立代理 |

---

## 二、Guardian 原始設計

### 2.1 核心功能

| 功能 | 說明 |
|------|------|
| **心跳檢測** | 定時向目標代理發送心跳，檢測存活狀態 |
| **熔斷器** | 當故障率超標時，斷開相關代理的連接 |
| **故障恢復** | 定期嘗試恢復被熔斷的連接 |

### 2.2 結構定義

```go
// Guardian 守護者結構
type Guardian struct {
    mu           sync.RWMutex
    agents       map[AgentID]*AgentMonitor
    heartbeatInterval time.Duration
    heartbeatTimeout  time.Duration
    circuitBreaker    *CircuitBreaker
    messageBus        *messagebus.MessageBus
    running         atomic.Bool
    heartbeatTimer   *time.Timer
}

type AgentMonitor struct {
    ID             AgentID
    LastHeartbeat  time.Time
    FailureCount   int
    State          MonitorState
    TasksCompleted int
    TasksFailed    int
}

type MonitorState string

const (
    MonitorStateHealthy   MonitorState = "healthy"
    MonitorStateDegraded  MonitorState = "degraded"
    MonitorStateFailed    MonitorState = "failed"
    MonitorStateIsolated  MonitorState = "isolated"
)
```

---

## 三、CircuitBreaker 熔斷器

### 3.1 熔斷器配置

```go
// CircuitBreakerConfig 熔斷器配置
type CircuitBreakerConfig struct {
    FailureThreshold  int           // 失敗次數閾值
    RecoveryTimeout   time.Duration // 恢復超時
    HalfOpenMaxCalls int           // 半開狀態最大嘗試次數
}

// DefaultCircuitBreakerConfig 預設配置
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
    FailureThreshold:  5,
    RecoveryTimeout:   30 * time.Second,
    HalfOpenMaxCalls: 3,
}
```

### 3.2 熔斷器狀態

```go
// CircuitState 熔斷器狀態
type CircuitState string

const (
    CircuitStateClosed   CircuitState = "closed"   // 關閉 (正常)
    CircuitStateOpen     CircuitState = "open"     // 打開 (熔斷)
    CircuitStateHalfOpen CircuitState = "half_open" // 半開 (嘗試恢復)
)
```

### 3.3 熔斷器結構

```go
// CircuitBreaker 熔斷器
type CircuitBreaker struct {
    mu           sync.Mutex
    state        CircuitState
    failures     int
    lastFailure  time.Time
    config       CircuitBreakerConfig
    isOpen       atomic.Bool // 標記是否打開
}
```

### 3.4 狀態轉換圖

```
                    ┌─────────┐
         ┌─────────►│ Closed  │◄─────────┐
         │          └────┬────┘           │
         │               │ failure         │ success
         │               ▼                 │
         │          ┌─────────┐            │
         │          │  Open   │────────────┘
         │          └────┬────┘
         │               │ recovery_timeout
         │               ▼
         │          ┌───────────┐
         └──────────│ Half-Open │
                    └─────┬─────┘
                          │
              ┌───────────┴───────────┐
              │ success               │ failure
              ▼                       ▼
         ┌─────────┐            ┌─────────┐
         │ Closed  │            │  Open   │
         └─────────┘            └─────────┘
```

---

## 四、GuardianExt 擴展設計

### 4.1 設計理念

將 Guardian 拆分為兩個獨立的子代理：

| 代理 | 職責 |
|------|------|
| **HeartbeatAgent** | 專門負責心跳監控 |
| **CircuitBreakerAgent** | 專門負責熔斷邏輯 |

### 4.2 AgentRole 類型定義

```go
// AgentRole 代理角色 (本地定義，避免循環依賴)
type AgentRole string

const (
    RoleCoordinator     AgentRole = "coordinator"
    RoleExecutor       AgentRole = "executor"
    RoleHeartbeat      AgentRole = "heartbeat"
    RoleCircuitBreaker AgentRole = "circuit_breaker"
    RoleResultAggregator AgentRole = "result_aggregator"
)
```

### 4.3 結構定義

```go
// GuardianExt 擴展守護者
type GuardianExt struct {
    mu             sync.RWMutex
    agents         map[AgentID]*AgentInfo
    heartbeatAgent *HeartbeatAgent
    circuitAgent   *CircuitBreakerAgent
    config         GuardianConfig
    messageBus     *messagebus.MessageBus
    stateMachine   *statemachine.StateMachine
}

type AgentInfo struct {
    ID        AgentID
    Role      AgentRole
    State     statemachine.AgentState
    CreatedAt time.Time
    LastSeen  time.Time
    Metadata  map[string]interface{}
}
```

### 4.4 核心方法

| 方法 | 說明 |
|------|------|
| `NewGuardianExt()` | 創建擴展守護者 |
| `RegisterAgent()` | 註冊代理 |
| `UnregisterAgent()` | 註銷代理 |
| `GetAgentInfo()` | 獲取代理資訊 |
| `ListAgents()` | 列出所有代理 |
| `MonitorAgent()` | 監控指定代理 |
| `IsolateAgent()` | 隔離問題代理 |

---

## 五、HeartbeatAgent 心跳代理

### 5.1 職責

| 職責 | 說明 |
|------|------|
| **心跳髮送** | 定時向所有代理發送心跳 |
| **心跳接收** | 接收並處理代理的心跳回應 |
| **超時檢測** | 檢測心跳超時的代理 |
| **狀態報告** | 向 Guardian 報告代理健康狀態 |

### 5.2 結構定義

```go
// HeartbeatAgent 心跳代理
type HeartbeatAgent struct {
    mu           sync.RWMutex
    agentID      AgentID
    interval     time.Duration
    timeout      time.Duration
    targets      map[AgentID]*HeartbeatTarget
    messageBus   *messagebus.MessageBus
    running      atomic.Bool
    ticker       *time.Ticker
    stats        HeartbeatStats
}

type HeartbeatTarget struct {
    AgentID     AgentID
    LastPing    time.Time
    LastPong    time.Time
    Latency     time.Duration
    FailureCount int
    State       HeartbeatState
}

type HeartbeatState string

const (
    HeartbeatStateAlive   HeartbeatState = "alive"
    HeartbeatStateUnknown HeartbeatState = "unknown"
    HeartbeatStateDead    HeartbeatState = "dead"
)
```

### 5.3 配置參數

| 參數 | 預設值 | 說明 |
|------|--------|------|
| `Interval` | 5s | 心跳間隔 |
| `Timeout` | 15s | 心跳超時 |
| `MaxFailures` | 3 | 最大失敗次數 |
| `RecoveryInterval` | 30s | 恢復檢測間隔 |

---

## 六、CircuitBreakerAgent 熔斷代理

### 6.1 職責

| 職責 | 說明 |
|------|------|
| **故障計數** | 統計代理的失敗次數 |
| **熔斷觸發** | 當故障超標時觸發熔斷 |
| **恢復檢測** | 定時檢測是否可以恢復 |
| **狀態同步** | 同步熔斷狀態到 MessageBus |

### 6.2 結構定義

```go
// CircuitBreakerAgent 熔斷代理
type CircuitBreakerAgent struct {
    mu           sync.RWMutex
    agentID      AgentID
    circuits     map[AgentID]*Circuit
    config       CircuitBreakerConfig
    messageBus   *messagebus.MessageBus
    stateMachine *statemachine.StateMachine
    running      atomic.Bool
}

type Circuit struct {
    Target       AgentID
    State        CircuitState
    Failures     int
    Successes    int
    LastFailure  time.Time
    LastStateChange time.Time
    HalfOpenCalls int
}
```

### 6.3 熔斷邏輯

```go
// shouldTrip 判斷是否應該熔斷
func (cb *Circuit) shouldTrip() bool {
    return cb.Failures >= cb.config.FailureThreshold
}

// shouldRecover 判斷是否應該恢復
func (cb *Circuit) shouldRecover() bool {
    return cb.State == CircuitStateOpen &&
           time.Since(cb.LastFailure) >= cb.config.RecoveryTimeout
}

// recordFailure 記錄失敗
func (cb *Circuit) recordFailure() {
    cb.Failures++
    cb.LastFailure = time.Now()
    
    if cb.shouldTrip() {
        cb.transitionTo(CircuitStateOpen)
    }
}

// recordSuccess 記錄成功
func (cb *Circuit) recordSuccess() {
    cb.Successes++
    
    if cb.State == CircuitStateHalfOpen {
        cb.HalfOpenCalls++
        if cb.HalfOpenCalls >= cb.config.HalfOpenMaxCalls {
            cb.transitionTo(CircuitStateClosed)
        }
    }
}
```

---

## 七、協作流程

### 7.1 心跳-熔斷協作圖

```
┌──────────────────────────────────────────────────────────────┐
│                      GuardianExt                             │
│  ┌─────────────────┐         ┌─────────────────────┐     │
│  │  HeartbeatAgent │─────────►│ CircuitBreakerAgent │     │
│  │   (心跳代理)     │ 故障報告 │    (熔斷代理)       │     │
│  └────────┬────────┘         └──────────┬──────────┘     │
│           │                              │                  │
│           │ heartbeat                    │ circuit state    │
│           │ status                       │                  │
│           ▼                              ▼                  │
│  ┌─────────────────────────────────────────────────────┐  │
│  │              MessageBus (消息總線)                    │  │
│  └─────────────────────────────────────────────────────┘  │
│           │                              │                  │
│           │                              │                  │
└───────────┼──────────────────────────────┼──────────────────┘
            │                              │
            ▼                              ▼
    ┌───────────────┐              ┌───────────────┐
    │    Agents     │              │    Swarm     │
    │  (被監控代理) │              │   (調度器)   │
    └───────────────┘              └───────────────┘
```

### 7.2 故障處理流程

```
1. HeartbeatAgent 檢測到心跳超時
   │
   ▼
2. HeartbeatAgent 向 CircuitBreakerAgent 報告故障
   │
   ▼
3. CircuitBreakerAgent 增加故障計數
   │
   ▼
4. 如果故障計數達到閾值:
   │  ├─ 觸發熔斷 (Open 狀態)
   │  ├─ 向 MessageBus 發布故障事件
   │  └─ 通知 Swarm 隔離該代理
   │
   ▼
5. 等待 RecoveryTimeout
   │
   ▼
6. CircuitBreakerAgent 進入 Half-Open 狀態
   │
   ▼
7. 允許有限數量的測試請求
   │
   ▼
8. 如果成功: 恢復 (Closed) / 如果失敗: 繼續熔斷 (Open)
```

---

## 八、配置範例

### 8.1 預設配置

```go
// GuardianConfig 守護者配置
type GuardianConfig struct {
    HeartbeatInterval    time.Duration
    HeartbeatTimeout     time.Duration
    MaxHeartbeatFailures int
    CircuitBreaker       CircuitBreakerConfig
}

var DefaultGuardianConfig = GuardianConfig{
    HeartbeatInterval:    5 * time.Second,
    HeartbeatTimeout:     15 * time.Second,
    MaxHeartbeatFailures: 3,
    CircuitBreaker:       DefaultCircuitBreakerConfig,
}
```

### 8.2 使用範例

```go
// 創建 GuardianExt
guardian := NewGuardianExt(messageBus, DefaultGuardianConfig)

// 註冊代理
guardian.RegisterAgent(&AgentInfo{
    ID:   "agent-1",
    Role: RoleExecutor,
})

// 啟動守護
guardian.Start()

// 清理
defer guardian.Stop()
```

---

## 九、錯誤處理

### 9.1 錯誤類型

```go
var (
    ErrAgentNotFound      = errors.New("agent not found")
    ErrAgentAlreadyExists = errors.New("agent already exists")
    ErrCircuitOpen        = errors.New("circuit breaker is open")
    ErrHeartbeatTimeout   = errors.New("heartbeat timeout")
    ErrInvalidConfig      = errors.New("invalid configuration")
)
```

### 9.2 恢復策略

| 錯誤 | 策略 |
|------|------|
| 網路瞬斷 | 自動重試 3 次 |
| 代理崩潰 | 觸發熔斷，隔離代理 |
| 資源耗盡 | 減少心跳頻率，等待恢復 |

---

## 十、測試策略

### 10.1 熔斷器測試

```go
func TestCircuitBreaker_Open(t *testing.T) {
    cb := NewCircuitBreaker(DefaultCircuitBreakerConfig)
    
    // 觸發熔斷
    for i := 0; i < 5; i++ {
        cb.RecordFailure()
    }
    
    assert.Equal(t, CircuitStateOpen, cb.State())
    assert.True(t, cb.isOpen.Load())
}

func TestCircuitBreaker_Recovery(t *testing.T) {
    cb := NewCircuitBreaker(DefaultCircuitBreakerConfig)
    cb.State = CircuitStateOpen
    
    // 等待恢復超時
    time.Sleep(35 * time.Second)
    
    // 嘗試恢復
    cb.TryRecover()
    
    assert.Equal(t, CircuitStateHalfOpen, cb.State())
}
```

### 10.2 心跳測試

```go
func TestHeartbeatAgent_DetectTimeout(t *testing.T) {
    agent := NewHeartbeatAgent(5 * time.Second)
    
    // 模擬目標代理
    target := &HeartbeatTarget{AgentID: "test-agent"}
    agent.targets["test-agent"] = target
    
    // 模擬超時
    target.LastPong = time.Now().Add(-20 * time.Second)
    
    // 檢測超時
    state := agent.checkTimeout(target)
    
    assert.Equal(t, HeartbeatStateDead, state)
}
```

---

## 十一、檔案位置

```
internal/agent/guardian/
├── guardian.go        # 原始守護者 (~590 行)
└── guardian_ext.go   # 擴展守護者 (~938 行)
```

---

## 十二、依賴關係

```
依賴:
    ├─ messagebus (消息總線)
    └─ statemachine (狀態機)

被依賴:
    ├─ SwarmExt (故障處理)
    └─ Coordinator (任務調度)
```

---

## 十三、擴展方向

| 擴展項 | 說明 |
|--------|------|
| **自適應閾值** | 根據歷史數據自動調整熔斷閾值 |
| **熔斷預警** | 在熔斷前發出預警 |
| **批量熔斷** | 支援批量隔離多個代理 |
| **熔斷策略** | 支援自定義熔斷策略 |

---

*文檔更新日期: 2026-04-04*
