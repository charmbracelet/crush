# SwarmExt 增強蜂群系統詳細設計

**版本**: v1.0  
**創建日期**: 2026-04-04  
**組件**: SwarmExt  
**狀態**: ✅ 已完成

---

## 一、組件概述

### 1.1 功能說明

SwarmExt 是 CrushCL 多代理系統的核心調度器，增強自原始 Swarm 實現，整合了 MessageBus、StateMachine、Guardian 等組件，提供完整的代理協作能力。

### 1.2 與原始 Swarm 的差異

| 特性 | 原始 Swarm | SwarmExt |
|------|-----------|----------|
| 消息通信 | 直接調用 | MessageBus |
| 狀態管理 | 簡單標記 | StateMachine |
| 故障處理 | 基本重試 | Guardian |
| 心跳檢測 | 無 | HeartbeatAgent |
| 熔斷機制 | 無 | CircuitBreakerAgent |

---

## 二、架構設計

### 2.1 系統架構圖

```
┌─────────────────────────────────────────────────────────────────┐
│                         SwarmExt                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Coordinator (協調器)                      │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐    │  │
│  │  │ Task    │  │ Agent   │  │ Resource│  │ Load   │    │  │
│  │  │Scheduler│  │Registry │  │Manager  │  │Balancer│    │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘    │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                    │
│  ┌───────────────────────────┼───────────────────────────────┐  │
│  │                           ▼                                │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │  │
│  │  │ MessageBus  │  │ StateMachine│  │  Guardian   │     │  │
│  │  │             │  │             │  │  Ext       │     │  │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │  │
│  └─────────┼────────────────┼────────────────┼─────────────┘  │
└─────────────┼────────────────┼────────────────┼────────────────┘
              │                │                │
              ▼                ▼                ▼
        ┌───────────┐    ┌───────────┐    ┌───────────┐
        │ Executor  │    │ Heartbeat │    │ Circuit   │
        │ Agents   │    │ Agent     │    │ Breaker   │
        └───────────┘    └───────────┘    └───────────┘
```

### 2.2 核心組件

| 組件 | 說明 |
|------|------|
| **Coordinator** | 任務協調器，負責任務分配和調度 |
| **AgentRegistry** | 代理註冊表，管理所有代理的生命週期 |
| **TaskScheduler** | 任務調度器，決定任務分配策略 |
| **ResourceManager** | 資源管理器，管理代理資源使用 |
| **LoadBalancer** | 負載均衡器，均衡代理工作負載 |

---

## 三、核心介面

### 3.1 SwarmExt 介面

```go
// SwarmExt 蜂群擴展介面
type SwarmExt interface {
    // Start 啟動蜂群
    Start() error
    
    // Stop 停止蜂群
    Stop() error
    
    // RegisterAgent 註冊代理
    RegisterAgent(agent *Agent) error
    
    // UnregisterAgent 註銷代理
    UnregisterAgent(id AgentID) error
    
    // SubmitTask 提交任務
    SubmitTask(task *Task) (*TaskResult, error)
    
    // SubmitTaskAsync 異步提交任務
    SubmitTaskAsync(task *Task, callback func(*TaskResult)) error
    
    // GetAgentStatus 獲取代理狀態
    GetAgentStatus(id AgentID) (*AgentStatus, error)
    
    // ListAgents 列出所有代理
    ListAgents() []*AgentInfo
    
    // Broadcast 廣播消息
    Broadcast(msg *Message) error
}
```

### 3.2 Task 結構

```go
// Task 任務結構
type Task struct {
    ID          TaskID
    Type        TaskType
    Payload     interface{}
    Priority    TaskPriority
    Timeout     time.Duration
    RetryPolicy *RetryPolicy
    Dependencies []TaskID
    Metadata    map[string]interface{}
}

type TaskType string

const (
    TaskTypeExecute   TaskType = "execute"
    TaskTypeQuery    TaskType = "query"
    TaskTypeMonitor  TaskType = "monitor"
    TaskTypeAggregate TaskType = "aggregate"
)

type TaskPriority int

const (
    PriorityLow    TaskPriority = 0
    PriorityNormal TaskPriority = 1
    PriorityHigh   TaskPriority = 2
    PriorityCritical TaskPriority = 3
)
```

### 3.3 TaskResult 結構

```go
// TaskResult 任務結果
type TaskResult struct {
    TaskID     TaskID
    Status     TaskStatus
    Output     interface{}
    Error      error
    StartedAt  time.Time
    CompletedAt time.Time
    Duration   time.Duration
    AgentID   AgentID
    Metadata  map[string]interface{}
}

type TaskStatus string

const (
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusTimeout   TaskStatus = "timeout"
    TaskStatusCanceled  TaskStatus = "canceled"
)
```

---

## 四、協調器設計

### 4.1 Coordinator 結構

```go
// Coordinator 協調器
type Coordinator struct {
    mu           sync.RWMutex
    agentID     AgentID
    tasks       map[TaskID]*Task
    taskQueue   *priorityqueue.PriorityQueue
    agentPool   *AgentPool
    scheduler   *TaskScheduler
    resourceMgr *ResourceManager
    loadBalancer *LoadBalancer
    messageBus  *messagebus.MessageBus
    stateMachine *statemachine.StateMachine
    running     atomic.Bool
}
```

### 4.2 任務調度流程

```
1. 提交任務 SubmitTask(task)
   │
   ▼
2. 驗證任務 (dependencies, payload)
   │
   ▼
3. 加入任務隊列 (按優先級排序)
   │
   ▼
4. 選擇最佳代理 SelectAgent()
   │  ├─ 檢查代理狀態 (StateMachine)
   │  ├─ 檢查資源使用 (ResourceManager)
   │  └─ 負載均衡 (LoadBalancer)
   │
   ▼
5. 分配任務 AssignTask()
   │
   ▼
6. 發送任務消息 (MessageBus.Send)
   │
   ▼
7. 等待結果或超時
   │
   ▼
8. 返回結果或錯誤
```

### 4.3 代理選擇策略

```go
// SelectAgent 選擇最佳代理
func (c *Coordinator) SelectAgent(task *Task) (*Agent, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    candidates := c.filterViableAgents(task)
    if len(candidates) == 0 {
        return nil, ErrNoAvailableAgent
    }
    
    // 按策略排序
    switch task.Priority {
    case PriorityCritical:
        return c.selectByCapability(candidates, task)
    case PriorityHigh:
        return c.selectByAvailability(candidates)
    default:
        return c.loadBalancer.Select(candidates)
    }
}

// filterViableAgents 過濾可用代理
func (c *Coordinator) filterViableAgents(task *Task) []*Agent {
    var result []*Agent
    for _, agent := range c.agentPool.All() {
        // 檢查狀態
        if !c.isAgentViable(agent) {
            continue
        }
        // 檢查能力匹配
        if !c.hasRequiredCapability(agent, task) {
            continue
        }
        // 檢查資源
        if !c.hasSufficientResources(agent, task) {
            continue
        }
        result = append(result, agent)
    }
    return result
}
```

---

## 五、代理池設計

### 5.1 AgentPool 結構

```go
// AgentPool 代理池
type AgentPool struct {
    mu       sync.RWMutex
    agents   map[AgentID]*Agent
    byRole   map[AgentRole][]AgentID
    byState  map[statemachine.AgentState][]AgentID
}

type Agent struct {
    ID        AgentID
    Role      AgentRole
    State    statemachine.AgentState
    Config   AgentConfig
    Stats    AgentStats
    Channel  chan *Message
    LastSeen time.Time
}

type AgentStats struct {
    TasksCompleted int
    TasksFailed   int
    AvgLatency    time.Duration
    CPUUsage      float64
    MemoryUsage   uint64
}
```

### 5.2 代理狀態追蹤

```go
// UpdateAgentState 更新代理狀態
func (ap *AgentPool) UpdateAgentState(id AgentID, state statemachine.AgentState) {
    ap.mu.Lock()
    defer ap.mu.Unlock()
    
    agent := ap.agents[id]
    oldState := agent.State
    agent.State = state
    agent.LastSeen = time.Now()
    
    // 更新狀態索引
    ap.byState[oldState] = removeAgent(ap.byState[oldState], id)
    ap.byState[state] = append(ap.byState[state], id)
}
```

---

## 六、負載均衡策略

### 6.1 支援的策略

| 策略 | 說明 |
|------|------|
| `LeastLoaded` | 選擇負載最輕的代理 |
| `RoundRobin` | 輪詢分配 |
| `Random` | 隨機選擇 |
| `Capability` | 按能力匹配 |
| `Affinity` | 親和性調度 |

### 6.2 策略實現

```go
// LoadBalancer 負載均衡器
type LoadBalancer struct {
    mu       sync.RWMutex
    strategy LoadBalanceStrategy
    stats    map[AgentID]*LoadStats
}

type LoadBalanceStrategy interface {
    Select(agents []*Agent) *Agent
}

type LeastLoaded struct{}

func (s *LeastLoaded) Select(agents []*Agent) *Agent {
    var best *Agent
    var minLoad = math.MaxFloat64
    
    for _, agent := range agents {
        load := s.calculateLoad(agent)
        if load < minLoad {
            minLoad = load
            best = agent
        }
    }
    return best
}

func (s *LeastLoaded) calculateLoad(agent *Agent) float64 {
    // 計算綜合負載分數
    taskScore := float64(agent.Stats.TasksRunning) * 10.0
    cpuScore := agent.Stats.CPUUsage * 5.0
    memScore := float64(agent.Stats.MemoryUsage) / float64(1024*1024*1024) * 2.0
    return taskScore + cpuScore + memScore
}
```

---

## 七、異步任務處理

### 7.1 異步提交流程

```go
// SubmitTaskAsync 異步提交任務
func (s *SwarmExt) SubmitTaskAsync(task *Task, callback func(*TaskResult)) error {
    // 生成回調通道
    callbackChan := make(chan *TaskResult, 1)
    
    // 啟動異步處理
    go func() {
        result, err := s.SubmitTask(task)
        if err != nil {
            result = &TaskResult{
                TaskID: task.ID,
                Status: TaskStatusFailed,
                Error:  err,
            }
        }
        
        // 執行回調
        if callback != nil {
            callback(result)
        }
    }()
    
    return nil
}
```

### 7.2 任務超時處理

```go
// SubmitTaskWithTimeout 帶超時的任務提交
func (s *SwarmExt) SubmitTaskWithTimeout(task *Task, timeout time.Duration) (*TaskResult, error) {
    resultChan := make(chan *TaskResult, 1)
    errorChan := make(chan error, 1)
    
    go func() {
        result, err := s.SubmitTask(task)
        if err != nil {
            errorChan <- err
            return
        }
        resultChan <- result
    }()
    
    select {
    case result := <-resultChan:
        return result, nil
    case err := <-errorChan:
        return nil, err
    case <-time.After(timeout):
        return &TaskResult{
            TaskID: task.ID,
            Status: TaskStatusTimeout,
        }, ErrTaskTimeout
    }
}
```

---

## 八、依賴管理

### 8.1 任務依賴圖

```go
// DependencyGraph 依賴圖
type DependencyGraph struct {
    mu      sync.RWMutex
    tasks   map[TaskID]*Task
    edges   map[TaskID][]TaskID
    inDegree map[TaskID]int
}

// AddTask 添加任務及其依賴
func (g *DependencyGraph) AddTask(task *Task) error {
    g.mu.Lock()
    defer g.mu.Unlock()
    
    g.tasks[task.ID] = task
    
    for _, depID := range task.Dependencies {
        g.edges[depID] = append(g.edges[depID], task.ID)
        g.inDegree[task.ID]++
    }
    
    return nil
}

// TopologicalSort 拓撲排序
func (g *DependencyGraph) TopologicalSort() ([]TaskID, error) {
    g.mu.RLock()
    defer g.mu.RUnlock()
    
    // Kahn 演算法
    queue := make([]TaskID, 0)
    for id, degree := range g.inDegree {
        if degree == 0 {
            queue = append(queue, id)
        }
    }
    
    var result []TaskID
    for len(queue) > 0 {
        id := queue[0]
        queue = queue[1:]
        result = append(result, id)
        
        for _, nextID := range g.edges[id] {
            g.inDegree[nextID]--
            if g.inDegree[nextID] == 0 {
                queue = append(queue, nextID)
            }
        }
    }
    
    if len(result) != len(g.tasks) {
        return nil, ErrCircularDependency
    }
    
    return result, nil
}
```

### 8.2 依賴任務執行

```go
// ExecuteWithDependencies 執行帶依賴的任務
func (s *SwarmExt) ExecuteWithDependencies(task *Task) (*TaskResult, error) {
    graph := NewDependencyGraph()
    graph.AddTask(task)
    
    // 拓撲排序
    order, err := graph.TopologicalSort()
    if err != nil {
        return nil, err
    }
    
    results := make(map[TaskID]*TaskResult)
    
    for _, taskID := range order {
        t := graph.tasks[taskID]
        
        // 等待依賴完成
        for _, depID := range t.Dependencies {
            if results[depID].Status != TaskStatusCompleted {
                return nil, ErrDependencyFailed
            }
            t.Payload = mergeResults(t.Payload, results[depID].Output)
        }
        
        // 執行任務
        result, err := s.SubmitTask(t)
        results[taskID] = result
        if err != nil {
            return result, err
        }
    }
    
    return results[task.ID], nil
}
```

---

## 九、配置與使用

### 9.1 SwarmExt 配置

```go
// Config SwarmExt 配置
type Config struct {
    MaxAgents         int
    MaxTasks          int
    TaskTimeout       time.Duration
    HeartbeatInterval time.Duration
    LoadBalanceStrategy string
    CoordinatorConfig  CoordinatorConfig
    GuardianConfig     guardian.GuardianConfig
}

var DefaultConfig = Config{
    MaxAgents:         10,
    MaxTasks:          100,
    TaskTimeout:       5 * time.Minute,
    HeartbeatInterval: 5 * time.Second,
    LoadBalanceStrategy: "least_loaded",
}
```

### 9.2 使用範例

```go
// 創建 SwarmExt
swarm, err := NewSwarmExt(&Config{
    MaxAgents: 5,
    CoordinatorConfig: CoordinatorConfig{
        MaxQueueSize: 100,
    },
})
if err != nil {
    log.Fatal(err)
}

// 註冊代理
swarm.RegisterAgent(&Agent{
    ID:   "agent-1",
    Role: RoleExecutor,
})

// 啟動
swarm.Start()

// 提交任務
result, err := swarm.SubmitTask(&Task{
    ID:       "task-1",
    Type:     TaskTypeExecute,
    Payload:  map[string]string{"command": "ls"},
    Priority: PriorityNormal,
})

fmt.Printf("Result: %+v\n", result)

// 關閉
swarm.Stop()
```

---

## 十、檔案位置

```
internal/agent/
├── swarm.go        # 原始蜂群 (~390 行)
└── swarm_ext.go   # 擴展蜂群 (~594 行)
```

---

## 十一、依賴關係

```
依賴:
    ├─ messagebus (消息總線)
    ├─ statemachine (狀態機)
    ├─ guardian/guardian_ext (守護者)
    ├─ coordinator (協調器)
    └─ Agent (代理)

被依賴:
    └─ cmd/hybrid-brain (混合大腦)
```

---

## 十二、擴展方向

| 擴展項 | 說明 |
|--------|------|
| **多集群支援** | 支援跨集群任務分配 |
| **任務預測** | 預測任務執行時間 |
| **自適應調度** | 根據歷史學習優化調度 |
| **任務分片** | 大任務自動分片並行 |

---

*文檔更新日期: 2026-04-04*
