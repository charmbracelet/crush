# ResultAggregator 結果聚合器

## 概述

ResultAggregator 是 CrushCL 多代理系統中的結果聚合元件，負責收集、整合和協調來自多個代理的執行結果。

## 設計目標

| 目標 | 說明 |
|------|------|
| **結果整合** | 收集多個代理的執行結果，統一輸出格式 |
| **衝突檢測** | 檢測並解決結果之間的衝突 |
| **優先級排序** | 根據代理優先級和結果品質排序 |
| **超時處理** | 處理代理結果超時情況 |
| **錯誤彙總** | 彙總多個代理的錯誤資訊 |

## 核心類型

### ResultAggregator 結構

```go
type ResultAggregator struct {
    mu           sync.RWMutex
    results      map[string]*TaskResult
    timeouts     map[string]time.Time
    config       AggregatorConfig
    collector    chan *TaskResult
    done         chan struct{}
}

type TaskResult struct {
    AgentID    string
    TaskID     string
    Status     ResultStatus
    Data       interface{}
    Err        error
    Priority   int
    Timestamp  time.Time
    Duration   time.Duration
    Metadata   map[string]interface{}
}

type ResultStatus int

const (
    ResultStatusPending ResultStatus = iota
    ResultStatusSuccess
    ResultStatusPartial
    ResultStatusFailed
    ResultStatusTimeout
)
```

## 配置參數

```go
type AggregatorConfig struct {
    Timeout         time.Duration  // 結果收集超時時間
    MaxResults      int            // 最大結果數量
    MinResults      int            // 最小需要結果數量
    PriorityEnabled bool           // 是否啟用優先級排序
    ConflictEnabled bool           // 是否啟用衝突檢測
    MergeStrategy   MergeStrategy   // 合併策略
}

type MergeStrategy int

const (
    MergeStrategyFirst MergeStrategy = iota  // 取第一個結果
    MergeStrategyLast                        // 取最後一個結果
    MergeStrategyPriority                    // 按優先級取
    MergeStrategyNewest                      // 取最新結果
    MergeStrategyAll                         // 合併所有結果
)
```

## 介面定義

```go
type ResultAggregatorInterface interface {
    // 提交結果
    Submit(result *TaskResult) error
    
    // 獲取聚合後結果
    Aggregate(taskID string) (*AggregatedResult, error)
    
    // 等待所有結果
    WaitAll(taskID string) ([]*TaskResult, error)
    
    // 獲取單一結果（按優先級）
    GetBest(taskID string) (*TaskResult, error)
    
    // 取消收集
    Cancel(taskID string)
    
    // 註冊超時回調
    OnTimeout(taskID string, callback func())
}
```

## 核心功能

### 1. 結果收集

```go
func (ra *ResultAggregator) Submit(result *TaskResult) error {
    ra.mu.Lock()
    defer ra.mu.Unlock()
    
    // 檢查是否已存在
    existing, ok := ra.results[result.TaskID]
    if ok && ra.config.PriorityEnabled {
        // 按優先級比較
        if existing.Priority > result.Priority {
            return nil // 保留更高優先級結果
        }
    }
    
    ra.results[result.TaskID] = result
    return nil
}
```

### 2. 超時處理

```go
func (ra *ResultAggregator) startTimeoutMonitor(taskID string, timeout time.Duration) {
    ra.mu.Lock()
    ra.timeouts[taskID] = time.Now().Add(timeout)
    ra.mu.Unlock()
    
    go func() {
        select {
        case <-time.After(timeout):
            ra.mu.Lock()
            if _, ok := ra.results[taskID]; !ok {
                // 超時，標記為超時狀態
                ra.results[taskID] = &TaskResult{
                    TaskID:    taskID,
                    Status:    ResultStatusTimeout,
                    Timestamp: time.Now(),
                }
            }
            ra.mu.Unlock()
        case <-ra.done:
            return
        }
    }()
}
```

### 3. 衝突檢測

```go
type Conflict struct {
    TaskID      string
    Results     []*TaskResult
    Resolution  ConflictResolution
    ResolvedBy  string
}

type ConflictResolution int

const (
    ConflictResolutionNone ConflictResolution = iota
    ConflictResolutionPriority
    ConflictResolutionMajority
    ConflictResolutionConsensus
)

func (ra *ResultAggregator) detectConflict(taskID string) (*Conflict, error) {
    ra.mu.RLock()
    defer ra.mu.RUnlock()
    
    results := ra.getResultsByTaskID(taskID)
    if len(results) < 2 {
        return nil, nil
    }
    
    // 檢查結果是否一致
    hashes := make(map[string]int)
    for _, r := range results {
        h := ra.hashResult(r)
        hashes[h]++
    }
    
    if len(hashes) > 1 {
        return &Conflict{
            TaskID:     taskID,
            Results:    results,
            Resolution: ConflictResolutionMajority,
        }, nil
    }
    
    return nil, nil
}

func (ra *ResultAggregator) hashResult(r *TaskResult) string {
    // 簡單的結果哈希
    dataStr := fmt.Sprintf("%v", r.Data)
    return fmt.Sprintf("%s:%s:%d", r.AgentID, dataStr, r.Status)
}
```

### 4. 結果聚合

```go
type AggregatedResult struct {
    TaskID       string
    Status       ResultStatus
    Primary      *TaskResult      // 主結果
    Alternatives []*TaskResult    // 備選結果
    Conflicts    []*Conflict       // 衝突資訊
    Summary     string           // 結果摘要
    Metadata    map[string]interface{}
}

func (ra *ResultAggregator) Aggregate(taskID string) (*AggregatedResult, error) {
    ra.mu.RLock()
    defer ra.mu.RUnlock()
    
    results := ra.getResultsByTaskID(taskID)
    if len(results) == 0 {
        return nil, ErrNoResults
    }
    
    // 按優先級排序
    sort.Slice(results, func(i, j int) bool {
        return results[i].Priority > results[j].Priority
    })
    
    // 檢測衝突
    conflicts := make([]*Conflict, 0)
    if ra.config.ConflictEnabled {
        if conflict := ra.detectConflictUnlocked(taskID); conflict != nil {
            conflicts = append(conflicts, conflict)
        }
    }
    
    // 確定主要結果
    primary := results[0]
    
    // 計算整體狀態
    status := ra.computeAggregatedStatus(results)
    
    return &AggregatedResult{
        TaskID:       taskID,
        Status:       status,
        Primary:      primary,
        Alternatives: results[1:],
        Conflicts:    conflicts,
        Summary:      ra.generateSummary(primary, conflicts),
        Metadata: map[string]interface{}{
            "total_results":  len(results),
            "conflict_count":  len(conflicts),
            "avg_duration":   ra.computeAvgDuration(results),
        },
    }, nil
}
```

### 5. 多結果等待

```go
func (ra *ResultAggregator) WaitAll(taskID string) ([]*TaskResult, error) {
    ra.mu.Lock()
    deadline, ok := ra.timeouts[taskID]
    if !ok {
        deadline = time.Now().Add(ra.config.Timeout)
    }
    ra.mu.Unlock()
    
    results := make([]*TaskResult, 0)
    
    for {
        ra.mu.RLock()
        results = ra.getResultsByTaskID(taskID)
        ra.mu.RUnlock()
        
        // 檢查是否滿足最小結果數
        if len(results) >= ra.config.MinResults {
            break
        }
        
        // 檢查是否超時
        if time.Now().After(deadline) {
            break
        }
        
        time.Sleep(10 * time.Millisecond)
    }
    
    return results, nil
}
```

## 使用範例

### 基本用法

```go
// 創建聚合器
aggregator := NewResultAggregator(AggregatorConfig{
    Timeout:         30 * time.Second,
    MaxResults:      10,
    MinResults:      2,
    PriorityEnabled: true,
    ConflictEnabled: true,
    MergeStrategy:   MergeStrategyPriority,
})

// 提交結果
aggregator.Submit(&TaskResult{
    AgentID:   "agent-1",
    TaskID:    "task-123",
    Status:    ResultStatusSuccess,
    Data:      map[string]string{"key": "value"},
    Priority:  10,
    Duration:  time.Second,
})

// 等待並聚合
results, _ := aggregator.WaitAll("task-123")
aggregated, _ := aggregator.Aggregate("task-123")

fmt.Printf("Status: %v\n", aggregated.Status)
fmt.Printf("Primary: %s\n", aggregated.Primary.AgentID)
```

### 與 SwarmExt 整合

```go
// 在 SwarmExt 中使用聚合器
type SwarmExtWithAggregator struct {
    *SwarmExt
    aggregator *ResultAggregator
}

func (s *SwarmExtWithAggregator) ExecuteTask(task *Task) (*AggregatedResult, error) {
    // 分發任務
    s.SwarmExt.Dispatch(task)
    
    // 收集結果
    results, err := s.aggregator.WaitAll(task.ID)
    if err != nil {
        return nil, err
    }
    
    // 聚合結果
    return s.aggregator.Aggregate(task.ID)
}
```

## 錯誤處理

```go
var (
    ErrNoResults       = errors.New("no results available")
    ErrTimeout         = errors.New("result collection timeout")
    ErrConflictUnresolved = errors.New("result conflict could not be resolved")
    ErrMaxResultsExceeded = errors.New("maximum results exceeded")
)

func (ra *ResultAggregator) Submit(result *TaskResult) error {
    ra.mu.Lock()
    defer ra.mu.Unlock()
    
    // 檢查最大結果數
    if ra.config.MaxResults > 0 {
        current := len(ra.results[result.TaskID])
        if current >= ra.config.MaxResults {
            return ErrMaxResultsExceeded
        }
    }
    
    // 設置時間戳
    if result.Timestamp.IsZero() {
        result.Timestamp = time.Now()
    }
    
    // 存儲結果
    ra.results[result.TaskID] = result
    
    return nil
}
```

## 線程安全

ResultAggregator 使用以下同步機制：

| 機制 | 用途 |
|------|------|
| `sync.RWMutex` | 保護結果映射表 |
| `sync.Map` | 可選的高併發結果存儲 |
| `atomic` | 計數器和標誌位 |
| `channel` | 結果收集和超時信號 |

## 性能優化

1. **結果緩存**：使用 LRU 緩存最近聚合的結果
2. **並行收集**：多個結果可並行提交
3. **延遲排序**：只在需要時才對結果進行排序
4. **批量處理**：支援批量提交和聚合

```go
type ResultAggregatorOptimized struct {
    *ResultAggregator
    cache   *lru.Cache[string, *AggregatedResult]
    batcher *Batcher[*TaskResult]
}
```

## 與其他元件的整合

| 元件 | 整合方式 |
|------|---------|
| MessageBus | 訂閱結果消息 |
| Guardian | 監控超時和錯誤 |
| StateMachine | 根據結果狀態轉換狀態 |
| SwarmExt | 收集分散式任務結果 |

## 下一步

- [ ] 實現 ResultAggregator 介面
- [ ] 添加結果驗證功能
- [ ] 實現衝突解決策略
- [ ] 添加結果持久化支援
