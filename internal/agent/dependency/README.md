# CrushCL DependencyManager 完整技術文檔

## 專案概覽

### 目標
為 CrushCL 實現一個任務依賴管理系統，支援：
- 任務依賴圖構建
- 循環依賴檢測
- 執行順序計算
- 並發安全操作
- 深度限制控制

### 位置
```
G:/AI分析/crushcl/internal/agent/dependency/
├── dependency.go      # 主實現 (754行)
└── dependency_test.go # 測試 (638行)
```

---

## 架構設計

### 核心結構

#### 1. TaskNode (任務節點)
```go
type TaskNode struct {
    TaskID       string                  // 任務ID
    State        NodeState               // 狀態
    Priority     int                     // 優先級
    DependsOn    []string                // 依賴的任務
    DependedBy   []string                // 依賴該任務的任務
    Metadata     map[string]interface{}   // 元數據
}
```

#### 2. NodeState (任務狀態)
```go
const (
    NodeStatePending   NodeState = iota  // 等待中
    NodeStateReady                       // 就緒
    NodeStateRunning                     // 執行中
    NodeStateCompleted                   // 已完成
    NodeStateFailed                     // 失敗
    NodeStateBlocked                    // 被阻塞
)
```

#### 3. DependencyGraph (依賴圖)
```go
type DependencyGraph struct {
    mu    sync.RWMutex
    nodes map[string]*TaskNode
    edges map[string][]string  // edges[from] = []to
}
```

#### 4. DependencyManager (依賴管理器)
```go
type DependencyManager struct {
    mu        sync.RWMutex
    graph     *DependencyGraph
    config    DependencyConfig
    listeners []DependencyListener
}
```

---

## 功能實現

### 1. 添加依賴 `AddDependency(taskID, dependsOn string)`

**流程**：
1. 檢查是否自己依賴自己 → 返回 `ErrInvalidDependency`
2. 檢查循環 → `wouldCreateCycle()` → 返回 `ErrCycleDetected`
3. 創建節點 → `getOrCreateNode()`
4. 添加邊 → `AddEdge()`
5. 檢查深度 → `getDependencyDepth()` → 返回 `ErrMaxDepthExceeded`
6. 發送事件 → `EventDependencyAdded`

**代碼位置**：`dependency.go:244-284`

### 2. 移除依賴 `RemoveDependency(taskID, dependsOn string)`

**流程**：
1. 檢查節點是否存在
2. 遍歷刪除 DependedBy 中的 taskID
3. 遍歷刪除 DependsOn 中的 dependsOn
4. 刪除 edges[dependsOn] 中的 taskID

**代碼位置**：`dependency.go:207-237`

### 3. 標記完成 `MarkCompleted(taskID string)`

**流程**：
1. 檢查節點存在
2. 更新狀態為 `NodeStateCompleted`
3. 遍歷所有依賴者，檢查是否全部完成
4. 如果全部完成 → 設置為 `NodeStateReady`
5. 發送事件 `EventTaskReady` 和 `EventTaskCompleted`

**代碼位置**：`dependency.go:488-527`

### 4. 標記失敗 `MarkFailed(taskID string, err error)`

**流程**：
1. 檢查節點存在
2. 更新狀態為 `NodeStateFailed`
3. 遞迴阻塞所有依賴者 → `blockDependentsRecursive()`
4. 發送事件 `EventTaskFailed`

**代碼位置**：`dependency.go:532-561`

### 5. 獲取就緒任務 `GetReadyTasks()`

**邏輯**：
- 返回所有 `State == NodeStateReady` 的任務
- 按 Priority 降序排序

**代碼位置**：`dependency.go:418-450`

### 6. 執行順序 `GetExecutionOrder()`

**邏輯**：
1. 計算所有節點的入度
2. 移除已完成的任務
3. 使用拓撲排序 (Kahn 算法)
4. 按 Priority 排序同級別任務
5. 檢測循環 → 返回 `ErrCycleDetected`

**代碼位置**：`dependency.go:604-712`

### 7. 循環檢測 `DetectCycles()`

**邏輯**：
- DFS 檢測圖中的循環
- 返回第一個發現的循環路徑

**代碼位置**：`dependency.go:551-580`

### 8. 深度計算 `getDependencyDepth(taskID string)`

**邏輯**：
- 遞迴計算最大依賴深度
- 使用 `visited` map 防止無限遞歸

**代碼位置**：`dependency.go:314-343`

---

## 配置項

```go
type DependencyConfig struct {
    MaxDepth           int           // 最大依賴深度
    Timeout            time.Duration // 依賴等待超時
    RetryEnabled       bool          // 是否啟用重試
    MaxRetries         int           // 最大重試次數
    ParallelEnabled    bool          // 是否啟用並行執行
    MaxParallelTasks   int           // 最大並行任務數
    CycleCheckEnabled  bool          // 是否啟用循環檢測
    AutoResolveEnabled bool          // 是否自動解決依賴
}
```

---

## 監聽器系統

```go
type DependencyListener interface {
    OnDependencyEvent(event DependencyEvent, taskID string, details ...interface{})
}

const (
    EventDependencyAdded  DependencyEvent = iota  // 添加依賴
    EventDependencyRemoved                        // 移除依賴
    EventTaskReady                               // 任務就緒
    EventTaskCompleted                           // 任務完成
    EventTaskFailed                              // 任務失敗
    EventTaskBlocked                             // 任務阻塞
    EventCycleDetected                           // 循環檢測
)
```

---

## 錯誤定義

```go
var (
    ErrTaskNotFound       = errors.New("task not found")
    ErrCycleDetected      = errors.New("cycle detected in dependency graph")
    ErrInvalidDependency  = errors.New("invalid dependency")
    ErrMaxDepthExceeded  = errors.New("maximum dependency depth exceeded")
    ErrTaskNotReady      = errors.New("task is not in ready state")
)
```

---

## Bug 修復記錄

### 1. `sort.Slice` 類型錯誤

**問題**：`sort.Slice` 的 comparator 函數接收的是索引，不是元素

**修復前**：
```go
sort.Slice(order, func(i, j *TaskNode) bool {
    return order[i].Priority > order[j].Priority
})
```

**修復後**：
```go
sort.Slice(order, func(i, j int) bool {
    return order[i].Priority > order[j].Priority
})
```

### 2. `MarkCompleted` 死鎖

**問題**：`notifyListeners` 內部嘗試獲取 `dm.mu.RLock()`，但調用者已持有 `dm.mu.Lock()`

**修復**：將 `notifyListeners` 移到 `Unlock()` 之後調用
```go
dm.mu.Unlock()

for _, dependentID := range node.DependedBy {
    if dependent, ok := dm.graph.nodes[dependentID]; ok && dependent.State == NodeStateReady {
        dm.notifyListeners(EventTaskReady, dependentID)
    }
}
dm.notifyListeners(EventTaskCompleted, taskID)
```

### 3. `GetReadyTasks` 狀態檢查不完整

**問題**：只檢查 `NodeStatePending`，但 `MarkCompleted` 設置狀態為 `NodeStateReady`

**修復**：同時檢查兩種狀態
```go
if node.State == NodeStatePending || node.State == NodeStateReady {
    if dm.canExecute(node) {
        ready = append(ready, node.TaskID)
    }
}
```

### 4. `MarkFailed` 未遞迴阻塞依賴者

**問題**：只阻塞直接依賴者，未遞迴處理下游

**修復**：新增 `blockDependentsRecursive` 函數
```go
func (dm *DependencyManager) blockDependentsRecursive(taskID string, blockErr error, blocked *[]string) {
    for _, dependentID := range dm.graph.nodes[taskID].DependedBy {
        if dm.graph.nodes[dependentID].State != NodeStateFailed {
            dm.graph.nodes[dependentID].State = NodeStateBlocked
            dm.graph.nodes[dependentID].Metadata["block_error"] = blockErr.Error()
            *blocked = append(*blocked, dependentID)
            dm.blockDependentsRecursive(dependentID, blockErr, blocked)
        }
    }
}
```

### 5. `DetectCycles` 嵌套鎖嘗試

**問題**：在已持有 `dm.graph.mu` 的情況下，內部函數再次嘗試獲取 `dm.graph.mu.RLock()`

**修復**：移除內部函數的鎖獲取，直接使用已持有的鎖

### 6. `getDependencyDepth` 循環依賴導致棧溢出

**問題**：未檢測已訪問節點，循環依賴導致無限遞歸

**修復**：新增 `visited` map 追蹤已訪問節點
```go
func (dm *DependencyManager) getDependencyDepthInternal(taskID string, visited map[string]bool) int {
    if visited[taskID] {
        return 0 // 循環依賴，返回 0 避免無限遞歸
    }
    visited[taskID] = true
    // ...
}
```

### 7. `AddDependency` 深度檢查順序錯誤

**問題**：在添加邊之前檢查深度，但邊尚未建立，計算結果不正確

**修復**：先添加邊，再檢查深度，最後回滾（如有必要）
```go
// 添加邊
dm.graph.AddEdge(dependsOn, taskID)

// 檢查最大深度 (在添加邊之後檢查)
if dm.config.MaxDepth > 0 {
    depth := dm.getDependencyDepth(taskID)
    if depth > dm.config.MaxDepth+1 {
        dm.graph.RemoveEdge(dependsOn, taskID) // 回滾
        return ErrMaxDepthExceeded
    }
}
```

---

## 測試覆蓋

### 測試結果：30/30 通過

| 測試 | 描述 |
|------|------|
| TestNewDependencyManager | 創建管理器 |
| TestAddDependency | 添加依賴 |
| TestAddDependencySelfReference | 自己依賴自己 |
| TestAddDependencyCycleDetection | 循環檢測 |
| TestAddDependencyCycleDetectionDisabled | 禁用循環檢測 |
| TestRemoveDependency | 移除依賴 |
| TestRemoveDependencyNotFound | 移除不存在的依賴 |
| TestCanExecute | 檢查可執行性 |
| TestCanExecuteTaskNotFound | 任務不存在 |
| TestGetReadyTasks | 獲取就緒任務 |
| TestGetReadyTasksWithPriority | 優先級排序 |
| TestMarkCompleted | 標記完成 |
| TestMarkCompletedNotFound | 完成不存在的任務 |
| TestMarkFailed | 標記失敗 |
| TestMarkFailedNotFound | 失敗不存在的任務 |
| TestDetectCycles | 檢測循環 |
| TestDetectCyclesNoCycle | 無循環情況 |
| TestGetExecutionOrder | 執行順序 |
| TestGetExecutionOrderWithPriority | 優先級順序 |
| TestGetExecutionOrderWithCycle | 循環圖順序 |
| TestListener | 監聽器基本功能 |
| TestListenerCycleDetection | 循環檢測事件 |
| TestGetState | 獲取狀態 |
| TestGetStateNotFound | 獲取不存在狀態 |
| TestGetMetadata | 獲取元數據 |
| TestToDOT | DOT導出 |
| TestConcurrentAccess | 並發訪問 |
| TestConcurrentMarkCompleted | 並發完成 |
| TestMaxDepthExceeded | 深度限制 |
| TestSetPriority | 設置優先級 |
| TestSetPriorityNotFound | 設置不存在優先級 |

---

## 關鍵設計決策

### 1. 鎖策略
- `DependencyManager` 級別使用 `sync.RWMutex`
- `DependencyGraph` 級別使用獨立的 `sync.RWMutex`
- 讀鎖(`RLock`)用於只讀操作
- 寫鎖(`Lock`)用於修改操作

### 2. 監聽器通知時機
- 監聽器在鎖釋放後通知，避免死鎖
- 使用可變參數 `details ...interface{}` 傳遞附加信息

### 3. 深度計算
- MaxDepth=3 允許 depth 0,1,2,3,4
- depth > MaxDepth+1 時才拒絕
- 循環依賴返回 depth=0，不阻塞添加但可被檢測

### 4. 自動節點創建
- `SetPriority` 自動創建不存在的節點
- `AddDependency` 會創建兩端點節點
- 簡化 API 使用，無需預先註冊任務

---

## 執行命令

```bash
# 編譯
cd "G:/AI分析/crushcl" && "G:/AI分析/go/bin/go.exe" build ./...

# 測試
cd "G:/AI分析/crushcl" && "G:/AI分析/go/bin/go.exe" test ./internal/agent/dependency/... -v

# 覆盖率
cd "G:/AI分析/crushcl" && "G:/AI分析/go/bin/go.exe" test ./internal/agent/dependency/... -cover
```

---

## 未來改進方向

1. **持久化支持** - 將依賴圖保存到磁盤
2. **超時機制** - 實現依賴等待超時
3. **重試機制** - 失敗任務自動重試
4. **優先級繼承** - 子任務繼承父任務優先級
5. **可視化** - 生成 Graphviz 圖形

---

*文檔創建時間：2026-04-05*
*維護者：Architect Agent*
