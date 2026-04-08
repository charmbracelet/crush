# DependencyManager Bug 修復總結

## 修復時間
2026-04-05

## 問題數量
共修復 **7 個 Bug**

---

## Bug 詳細

### 1. sort.Slice 類型錯誤 ❌→✅

**位置**：`dependency.go` (約 line 400)

**問題**：
```go
// 錯誤：Comparator 參數是 int，不是元素類型
sort.Slice(order, func(i, j *TaskNode) bool {
    return order[i].Priority > order[j].Priority
})
```

**修復**：
```go
// 正確：參數是 int
sort.Slice(order, func(i, j int) bool {
    return order[i].Priority > order[j].Priority
})
```

---

### 2. MarkCompleted 死鎖 ❌→✅

**位置**：`dependency.go:487-489`

**問題**：`notifyListeners` 在已持有鎖的情況下嘗試再次獲取鎖

**修復**：將 notifyListeners 移到 Unlock 之後
```go
dm.mu.Unlock()

for _, dependentID := range node.DependedBy {
    if dependent, ok := dm.graph.nodes[dependentID]; ok && dependent.State == NodeStateReady {
        dm.notifyListeners(EventTaskReady, dependentID)
    }
}
dm.notifyListeners(EventTaskCompleted, taskID)
```

---

### 3. GetReadyTasks 狀態檢查不完整 ❌→✅

**位置**：`dependency.go:422`

**問題**：只檢查 `NodeStatePending`，但 `MarkCompleted` 設置為 `NodeStateReady`

**修復**：同時檢查兩種狀態
```go
if node.State == NodeStatePending || node.State == NodeStateReady {
    if dm.canExecute(node) {
        ready = append(ready, node.TaskID)
    }
}
```

---

### 4. MarkFailed 未遞迴阻塞下游 ❌→✅

**位置**：`dependency.go:532-561`

**問題**：只阻塞直接依賴者，沒有處理下游

**修復**：新增 `blockDependentsRecursive` 函數
```go
func (dm *DependencyManager) blockDependentsRecursive(taskID string, blockErr error, blocked *[]string) {
    for _, dependentID := range dm.graph.nodes[taskID].DependedBy {
        if dm.graph.nodes[dependentID].State != NodeStateFailed {
            dm.graph.nodes[dependentID].State = NodeStateBlocked
            *blocked = append(*blocked, dependentID)
            dm.blockDependentsRecursive(dependentID, blockErr, blocked)
        }
    }
}
```

---

### 5. DetectCycles 嵌套鎖 ❌→✅

**位置**：`dependency.go:551-580`

**問題**：在已持有 `dm.graph.mu` 的情況下，內部函數再次嘗試獲取鎖

**修復**：移除內部函數的鎖獲取調用

---

### 6. getDependencyDepth 循環依賴棧溢出 ❌→✅

**位置**：`dependency.go:314-343`

**問題**：未檢測已訪問節點，循環依賴導致無限遞歸

**修復**：新增 `visited` map
```go
func (dm *DependencyManager) getDependencyDepthInternal(taskID string, visited map[string]bool) int {
    if visited[taskID] {
        return 0 // 循環依賴，返回 0 避免無限遞歸
    }
    visited[taskID] = true
    // ...
}
```

---

### 7. AddDependency 深度檢查順序錯誤 ❌→✅

**位置**：`dependency.go:244-284`

**問題**：在添加邊之前檢查深度，計算結果不正確

**修復**：先添加邊，再檢查，最後回滾（如有必要）
```go
dm.graph.AddEdge(dependsOn, taskID)

if dm.config.MaxDepth > 0 {
    depth := dm.getDependencyDepth(taskID)
    if depth > dm.config.MaxDepth+1 {
        dm.graph.RemoveEdge(dependsOn, taskID) // 回滾
        return ErrMaxDepthExceeded
    }
}
```

---

## 新增功能

### RemoveEdge 方法
```go
func (dg *DependencyGraph) RemoveEdge(from, to string)
```
用於深度超限時回滾操作

---

## 測試修復

| 測試 | 問題 | 修復 |
|------|------|------|
| TestListenerCycleDetection | 字串切片 `e[:13]` 應為 `e[:14]` | 已修正 |
| TestConcurrentMarkCompleted | `string(rune('0'+i))` i≥10 時錯誤 | 改用 `fmt.Sprintf` |
| TestSetPriorityNotFound | 測試期望自動創建任務 | 修改 SetPriority 自動創建 |
| TestMaxDepthExceeded | `>` 比較導致提前失敗 | 改為 `> MaxDepth+1` |

---

## 最終測試結果

```
=== RUN   TestNewDependencyManager
--- PASS: TestNewDependencyManager (0.00s)
=== RUN   TestAddDependency
--- PASS: TestAddDependency (0.00s)
... (30 tests total)
--- PASS: TestSetPriorityNotFound (0.00s)
PASS
ok  	github.com/charmbracelet/crushcl/internal/agent/dependency	0.549s
```

**30/30 測試全部通過**

---

*修復完成時間：2026-04-05*
