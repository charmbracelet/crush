# DependencyManager 依賴管理器

## 概述

DependencyManager 是 CrushCL 多代理系統中的依賴管理元件，負責追蹤、管理和調度代理之間的任務依賴關係。

## 設計目標

| 目標 | 說明 |
|------|------|
| **依賴追蹤** | 記錄任務之間的依賴關係 |
| **循環檢測** | 檢測並防止依賴循環 |
| **調度優化** | 根據依賴關係優化執行順序 |
| **並行化** | 識別可並行執行的任務 |
| **死鎖預防** | 預防和解決依賴死鎖 |

## 核心類型

### DependencyManager 結構

```go
type DependencyManager struct {
    mu          sync.RWMutex
    graph       *DependencyGraph
    scheduler   *TaskScheduler
    executors   map[string]TaskExecutor
    listeners   []DependencyListener
    config      DependencyConfig
}

type DependencyGraph struct {
    nodes map[string]*TaskNode
    edges map[string][]string // adjacency list
    mu    sync.RWMutex
}

type TaskNode struct {
    TaskID      string
    DependsOn   []string
    DependedBy  []string
    State       NodeState
    Priority    int
    Metadata    map[string]interface{}
}

type NodeState int

const (
    NodeStatePending NodeState = iota
    NodeStateReady
    NodeStateRunning
    NodeStateCompleted
    NodeStateFailed
    NodeStateBlocked
)
```

## 配置參數

```go
type DependencyConfig struct {
    MaxDepth           int              // 最大依賴深度
    Timeout            time.Duration    // 依賴等待超時
    RetryEnabled       bool             // 是否啟用重試
    MaxRetries         int              // 最大重試次數
    ParallelEnabled    bool             // 是否啟用並行執行
    MaxParallelTasks   int              // 最大並行任務數
    CycleCheckEnabled  bool             // 是否啟用循環檢測
    AutoResolveEnabled bool            // 是否自動解決依賴
}
```

## 介面定義

```go
type DependencyManagerInterface interface {
    // 添加任務依賴
    AddDependency(taskID, dependsOn string) error
    
    // 移除任務依賴
    RemoveDependency(taskID, dependsOn string) error
    
    // 獲取任務的依賴項
    GetDependencies(taskID string) ([]string, error)
    
    // 獲取依賴該任務的任務
    GetDependents(taskID string) ([]string, error)
    
    // 檢查任務是否可以執行
    CanExecute(taskID string) (bool, error)
    
    // 獲取可執行的任務列表
    GetReadyTasks() ([]string, error)
    
    // 標記任務完成
    MarkCompleted(taskID string) error
    
    // 標記任務失敗
    MarkFailed(taskID string, err error) error
    
    // 檢測循環依賴
    DetectCycles() ([]string, error)
    
    // 獲取執行順序
    GetExecutionOrder() ([]string, error)
}
```

## 核心功能

### 1. 添加依賴

```go
func (dm *DependencyManager) AddDependency(taskID, dependsOn string) error {
    dm.mu.Lock()
    defer dm.mu.Unlock()
    
    // 檢查是否會造成循環
    if dm.config.CycleCheckEnabled {
        if dm.wouldCreateCycle(dependsOn, taskID) {
            return ErrCycleDetected
        }
    }
    
    // 創建任務節點（如果不存在）
    dm.getOrCreateNode(taskID)
    dm.getOrCreateNode(dependsOn)
    
    // 添加邊
    dm.graph.AddEdge(dependsOn, taskID)
    
    // 觸發事件
    dm.notifyListeners(EventDependencyAdded, taskID, dependsOn)
    
    return nil
}

func (dg *DependencyGraph) AddEdge(from, to string) {
    dg.mu.Lock()
    defer dg.mu.Unlock()
    
    // 添加從 -> 到 的邊
    if dg.edges[from] == nil {
        dg.edges[from] = []string{}
    }
    dg.edges[from] = append(dg.edges[from], to)
    
    // 更新節點
    dg.nodes[from].DependedBy = append(dg.nodes[from].DependedBy, to)
    dg.nodes[to].DependsOn = append(dg.nodes[to].DependsOn, from)
}
```

### 2. 循環檢測

```go
func (dm *DependencyManager) wouldCreateCycle(from, to string) bool {
    // 檢查從 'to' 出發是否能到達 'from'
    visited := make(map[string]bool)
    return dm.canReach(to, from, visited)
}

func (dm *DependencyManager) canReach(from, to string, visited map[string]bool) bool {
    if from == to {
        return true
    }
    if visited[from] {
        return false
    }
    visited[from] = true
    
    dm.graph.mu.RLock()
    edges := dm.graph.edges[from]
    dm.graph.mu.RUnlock()
    
    for _, neighbor := range edges {
        if dm.canReach(neighbor, to, visited) {
            return true
        }
    }
    return false
}

func (dm *DependencyManager) DetectCycles() ([]string, error) {
    dm.graph.mu.Lock()
    defer dm.graph.mu.Unlock()
    
    var cycles []string
    visited := make(map[string]bool)
    recStack := make(map[string]bool)
    
    for nodeID := range dm.graph.nodes {
        if !visited[nodeID] {
            path := make([]string, 0)
            if dm.detectCycleDFS(nodeID, visited, recStack, &path) {
                cycles = append(cycles, strings.Join(path, " -> "))
            }
        }
    }
    
    return cycles, nil
}

func (dm *DependencyManager) detectCycleDFS(nodeID string, visited, recStack map[string]bool, path *[]string) bool {
    visited[nodeID] = true
    recStack[nodeID] = true
    *path = append(*path, nodeID)
    
    dm.graph.mu.RLock()
    edges := dm.graph.edges[nodeID]
    dm.graph.mu.RUnlock()
    
    for _, neighbor := range edges {
        if !visited[neighbor] {
            if dm.detectCycleDFS(neighbor, visited, recStack, path) {
                return true
            }
        } else if recStack[neighbor] {
            *path = append(*path, neighbor)
            return true
        }
    }
    
    *path = *path[:len(*path)-1]
    recStack[nodeID] = false
    return false
}
```

### 3. 執行就緒判斷

```go
func (dm *DependencyManager) CanExecute(taskID string) (bool, error) {
    dm.mu.RLock()
    defer dm.mu.RUnlock()
    
    node, ok := dm.graph.nodes[taskID]
    if !ok {
        return false, ErrTaskNotFound
    }
    
    // 檢查依賴是否都已完成
    for _, depID := range node.DependsOn {
        depNode, ok := dm.graph.nodes[depID]
        if !ok {
            continue // 依賴的任務不存在，視為已完成
        }
        
        if depNode.State != NodeStateCompleted {
            return false, nil
        }
    }
    
    return true, nil
}

func (dm *DependencyManager) GetReadyTasks() ([]string, error) {
    dm.mu.RLock()
    defer dm.mu.RUnlock()
    
    var ready []string
    
    for nodeID, node := range dm.graph.nodes {
        if node.State != NodeStatePending {
            continue
        }
        
        // 檢查所有依賴是否都已完成
        allDepsCompleted := true
        for _, depID := range node.DependsOn {
            if depNode, ok := dm.graph.nodes[depID]; ok {
                if depNode.State != NodeStateCompleted {
                    allDepsCompleted = false
                    break
                }
            }
        }
        
        if allDepsCompleted {
            ready = append(ready, nodeID)
        }
    }
    
    // 按優先級排序
    sort.Slice(ready, func(i, j string) bool {
        return dm.graph.nodes[i].Priority > dm.graph.nodes[j].Priority
    })
    
    return ready, nil
}
```

### 4. 任務完成/失敗處理

```go
func (dm *DependencyManager) MarkCompleted(taskID string) error {
    dm.mu.Lock()
    defer dm.mu.Unlock()
    
    node, ok := dm.graph.nodes[taskID]
    if !ok {
        return ErrTaskNotFound
    }
    
    node.State = NodeStateCompleted
    
    // 檢查是否有依賴該任務的任務變為就緒
    for _, dependentID := range node.DependedBy {
        dependent, ok := dm.graph.nodes[dependentID]
        if !ok {
            continue
        }
        
        if dependent.State == NodeStatePending {
            // 檢查所有依賴是否都已完成
            allDone := true
            for _, depID := range dependent.DependsOn {
                if dm.graph.nodes[depID].State != NodeStateCompleted {
                    allDone = false
                    break
                }
            }
            
            if allDone {
                dependent.State = NodeStateReady
            }
        }
    }
    
    dm.notifyListeners(EventTaskCompleted, taskID)
    return nil
}

func (dm *DependencyManager) MarkFailed(taskID string, err error) error {
    dm.mu.Lock()
    defer dm.mu.Unlock()
    
    node, ok := dm.graph.nodes[taskID]
    if !ok {
        return ErrTaskNotFound
    }
    
    node.State = NodeStateFailed
    
    // 標記所有依賴該任務的任務為 blocked
    for _, dependentID := range node.DependedBy {
        dependent, ok := dm.graph.nodes[dependentID]
        if !ok {
            continue
        }
        
        if dependent.State == NodeStatePending || dependent.State == NodeStateReady {
            dependent.State = NodeStateBlocked
            dependent.Metadata["blockReason"] = fmt.Sprintf("dependency %s failed: %v", taskID, err)
        }
    }
    
    dm.notifyListeners(EventTaskFailed, taskID, err)
    return nil
}
```

### 5. 執行順序計算

```go
func (dm *DependencyManager) GetExecutionOrder() ([]string, error) {
    dm.mu.RLock()
    defer dm.mu.RUnlock()
    
    // 使用拓撲排序
    inDegree := make(map[string]int)
    for nodeID := range dm.graph.nodes {
        inDegree[nodeID] = 0
    }
    
    // 計算入度
    for _, edges := range dm.graph.edges {
        for _, to := range edges {
            inDegree[to]++
        }
    }
    
    // 找到所有入度為 0 的節點
    var queue []string
    for nodeID, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, nodeID)
        }
    }
    
    // 按優先級排序初始節點
    sort.Slice(queue, func(i, j string) bool {
        return dm.graph.nodes[i].Priority > dm.graph.nodes[j].Priority
    })
    
    var result []string
    
    for len(queue) > 0 {
        // 取出隊首
        current := queue[0]
        queue = queue[1:]
        result = append(result, current)
        
        // 處理所有依賴該節點的節點
        dependentIDs := dm.graph.nodes[current].DependedBy
        sort.Slice(dependentIDs, func(i, j string) bool {
            return dm.graph.nodes[i].Priority > dm.graph.nodes[j].Priority
        })
        
        for _, depID := range dependentIDs {
            inDegree[depID]--
            if inDegree[depID] == 0 {
                queue = append(queue, depID)
            }
        }
    }
    
    // 檢查是否有循環
    if len(result) != len(dm.graph.nodes) {
        return nil, ErrCycleDetected
    }
    
    return result, nil
}
```

## 事件監聽

```go
type DependencyEvent int

const (
    EventDependencyAdded DependencyEvent = iota
    EventDependencyRemoved
    EventTaskReady
    EventTaskCompleted
    EventTaskFailed
    EventTaskBlocked
    EventCycleDetected
)

type DependencyListener interface {
    OnDependencyEvent(event DependencyEvent, taskID string, details ...interface{})
}

type DependencyManager withListeners struct {
    *DependencyManager
    listeners []DependencyListener
}

func (dm *DependencyManager) notifyListeners(event DependencyEvent, taskID string, details ...interface{}) {
    for _, listener := range dm.listeners {
        listener.OnDependencyEvent(event, taskID, details...)
    }
}
```

## 使用範例

### 基本用法

```go
// 創建依賴管理器
dm := NewDependencyManager(DependencyConfig{
    MaxDepth:          10,
    Timeout:           5 * time.Minute,
    RetryEnabled:      true,
    MaxRetries:        3,
    ParallelEnabled:   true,
    MaxParallelTasks:  5,
    CycleCheckEnabled: true,
})

// 添加任務依賴
dm.AddDependency("task-B", "task-A") // B 依賴 A
dm.AddDependency("task-C", "task-A") // C 依賴 A
dm.AddDependency("task-D", "task-B") // D 依賴 B
dm.AddDependency("task-D", "task-C") // D 依賴 B 和 C

// 獲取執行順序
order, err := dm.GetExecutionOrder()
// ["task-A", "task-B", "task-C", "task-D"] 或 ["task-A", "task-C", "task-B", "task-D"]

// 獲取可執行的任務
ready, _ := dm.GetReadyTasks()
// ["task-A"]

// 標記任務完成
dm.MarkCompleted("task-A")

// 再次獲取可執行任務
ready, _ = dm.GetReadyTasks()
// ["task-B", "task-C"] (可並行)
```

### 與 SwarmExt 整合

```go
type SwarmExtWithDeps struct {
    *SwarmExt
    depManager *DependencyManager
}

func (s *SwarmExtWithDeps) SubmitTaskWithDeps(task *Task, deps []string) error {
    // 添加任務
    s.SwarmExt.Submit(task)
    
    // 添加依賴
    for _, dep := range deps {
        s.depManager.AddDependency(task.ID, dep)
    }
    
    // 如果任務就緒，開始執行
    if canExec, _ := s.depManager.CanExecute(task.ID); canExec {
        return s.SwarmExt.Execute(task.ID)
    }
    
    return nil
}

func (s *SwarmExtWithDeps) OnTaskComplete(taskID string, result interface{}) {
    s.depManager.MarkCompleted(taskID)
    
    // 檢查是否有任務現在可以執行
    ready, _ := s.depManager.GetReadyTasks()
    for _, readyTaskID := range ready {
        s.SwarmExt.Execute(readyTaskID)
    }
}
```

## 錯誤處理

```go
var (
    ErrTaskNotFound    = errors.New("task not found in dependency graph")
    ErrCycleDetected   = errors.New("dependency cycle detected")
    ErrInvalidDependency = errors.New("invalid dependency relationship")
    ErrMaxDepthExceeded = errors.New("maximum dependency depth exceeded")
    ErrCircularDependency = errors.New("circular dependency detected")
)
```

## 性能優化

1. **延遲圖構建**：只在需要時才構建完整圖結構
2. **快取就緒列表**：緩存可執行任務列表
3. **增量更新**：只更新受影響的節點狀態
4. **並行圖遍歷**：利用 goroutine 並行處理獨立的子圖

```go
type DependencyManagerOptimized struct {
    *DependencyManager
    readyCache    *TTLCache[string, []string]
    graphSnapshot *DependencyGraph
    updateCh      chan struct{}
}

func (dm *DependencyManagerOptimized) invalidateCache() {
    dm.readyCache.Invalidate()
}
```

## 與其他元件的整合

| 元件 | 整合方式 |
|------|---------|
| SwarmExt | 分發任務時建立依賴關係 |
| ResultAggregator | 根據依賴關係聚合結果 |
| Guardian | 監控依賴超時和死鎖 |
| StateMachine | 根據依賴狀態更新狀態機 |

## 下一步

- [ ] 實現 DependencyManager 介面
- [ ] 添加視覺化依賴圖功能
- [ ] 實現死鎖自動檢測和恢復
- [ ] 添加依賴效能分析工具
