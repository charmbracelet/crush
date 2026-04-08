# CrushCL 功能實現完善計劃書

**版本**: 1.0  
**日期**: 2026-04-04  
**基於**: `HYBRID_BRAIN_PLAN_v1.0.md`  
**狀態**: 待執行

---

## 1. 概述

### 1.1 目標

完成 CrushCL 的 HybridBrain 核心功能，實現 CL/CC 智能路由，使系統能夠：
- 自動分類任務複雜度
- 根據預算和性能選擇最優執行路徑
- 提供 HTTP API 供外部調用

### 1.2 階段規劃

| 階段 | 週期 | 主要目標 |
|------|------|----------|
| Phase 0 | 1天 | 緊急修復 (Log Leakage, Config) |
| Phase 1 | 3-5天 | HybridBrain 核心實現 |
| Phase 2 | 2-3天 | HTTP API Server |
| Phase 3 | 2-3天 | CLI 整合 |
| Phase 4 | 3-5天 | 測試與文檔 |

**總工期**: 約 11-17 天

---

## 2. Phase 0: 緊急修復 (Day 0)

### 2.1 修復 Log Leakage

**檔案**: `internal/cmd/root.go:162`

**問題**: `config.Load` 在 logger 設置前調用 `slog`，導致日誌泄漏

**方案**: 
```go
// 修改 config.Load，返回 warnings 而非直接 slog
func Load(...) (cfg *Config, diagnostics []string, err error) {
    // 移除 slog 調用
    // 改為返回 diagnostics
}
```

### 2.2 修復 Config Mutation

**檔案**: `internal/ui/dialog/models.go:487`

**問題**: 在讀取操作中突變配置

**方案**: 將 filtering logic 移到寫入時

### 2.3 實現 HybridBrain.Execute 橋接

**檔案**: `internal/kernel/server/http_server.go:184`

**問題**: TODO 指出應調用 `HybridBrain.Execute()` 但未實現

**方案**: 
1. 創建 `HybridBrain` interface
2. 實現橋接到現有 `executeTask()`
3. 預留 `HybridBrain.Execute()` 擴展點

---

## 3. Phase 1: HybridBrain 核心實現 (Day 1-5)

### 3.1 實現 CL Kernel Client

**目標**: 實現與 CrushCL 內核的通信

**檔案**: `internal/kernel/cl_kernel/client.go` (新建)

```go
package cl_kernel

type CLKernelClient interface {
    Execute(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error)
    ExecuteStream(ctx context.Context, req ExecuteRequest, stream func(chunk string) error) error
    GetSession(ctx context.Context, sessionID string) (*SessionState, error)
    UpdateSession(ctx context.Context, sessionID string, update SessionUpdate) error
    Close() error
}

type ExecuteRequest struct {
    Prompt   string
    Tools    []string
    Model    string
    Stream   bool
    SessionID string
}

type ExecutionResult struct {
    Output       string
    ExecutorType string  // "cl" | "cc" | "hybrid"
    Tokens       int
    CostUSD      float64
    DurationMs   int64
    Error        error
}
```

**實現步驟**:
1. 創建 `cl_kernel/` 目錄結構
2. 實現 HTTP Client
3. 實現流式響應處理
4. 實現 Session 管理
5. 添加 Unit Tests

### 3.2 實現 Coordination 模組

**目標**: 任務調度、資源管理、負載均衡

#### 3.2.1 Task Scheduler

**檔案**: `internal/kernel/coordination/task_scheduler.go`

```go
type TaskScheduler struct {
    queue    chan *Task
    priority PriorityQueue
    workers  int
}

type Task struct {
    ID          string
    Type        TaskType
    Prompt      string
    Tools       []string
    Priority    int
    BudgetUSD   float64
    CreatedAt   time.Time
    Result      *ExecutionResult
    callbacks   []func(*Task)
}

type TaskType int
const (
    TaskUnknown TaskType = iota
    TaskQuickLookup
    TaskFileOperation
    TaskDataProcessing
    TaskGitHub
    TaskComplexRefactor
    TaskCodeReview
    TaskBugHunt
    TaskCreative
    TaskMCPTask
)
```

#### 3.2.2 Resource Manager

**檔案**: `internal/kernel/coordination/resource_manager.go`

```go
type ResourceManager struct {
    mutex           sync.RWMutex
    budgetUSD       float64
    budgetLock      sync.Mutex
    usedBudgetUSD   float64
    activeTasks     map[string]*Task
    maxConcurrent   int
}

func (rm *ResourceManager) ReserveBudget(taskID string, amount float64) error
func (rm *ResourceManager) ReleaseBudget(taskID string) error
func (rm *ResourceManager) GetBudget() (remaining, used float64)
func (rm *ResourceManager) AcquireSlot(taskID string) error
func (rm *ResourceManager) ReleaseSlot(taskID string)
```

#### 3.2.3 Load Balancer

**檔案**: `internal/kernel/coordination/load_balancer.go`

```go
type LoadBalancer struct {
    clClient  *CLKernelClient
    ccBridge  *CCBridge
    scheduler *TaskScheduler
    resources *ResourceManager
}

func (lb *LoadBalancer) SelectExecutor(task *Task) ExecutorType
func (lb *LoadBalancer) Execute(task *Task) (*ExecutionResult, error)
```

### 3.3 實現 Task Classifier

**目標**: 自動分類任務類型

**檔案**: `internal/kernel/coordination/task_classifier.go`

```go
type TaskClassifier struct {
    // 基於關鍵詞的簡單分類
    patterns map[TaskType][]*regexp.Regexp
    // 預訓練權重（可擴展）
}

func (tc *TaskClassifier) Classify(prompt string) TaskType {
    // 1. 關鍵詞匹配
    // 2. 模式識別
    // 3. 返回最可能類型
}
```

**分類規則**:
| TaskType | 關鍵詞 |
|----------|--------|
| TaskQuickLookup | what is, define, explain, how does |
| TaskFileOperation | read, write, edit, file, list |
| TaskGitHub | commit, pr, branch, merge, github |
| TaskBugHunt | bug, error, fix, debug, issue |
| TaskCodeReview | review, security, scan, check |
| TaskCreative | brainstorm, idea, suggest, creative |

### 3.4 實現 Cost Optimizer

**目標**: 估算任務成本，決策執行者

**檔案**: `internal/kernel/coordination/cost_optimizer.go`

```go
type CostOptimizer struct {
    modelCosts map[string]ModelCost
    clBaseCost float64  // 本地執行基礎成本
}

type ModelCost struct {
    InputPer1M  float64
    OutputPer1M float64
    CacheHit    float64
}

func (co *CostOptimizer) Estimate(task *Task, executor ExecutorType) float64
func (co *CostOptimizer) ShouldDowngrade(task *Task, remainingBudget float64) bool
```

---

## 4. Phase 2: HTTP API Server (Day 6-8)

### 4.1 Server 框架

**檔案**: `internal/kernel/server/server.go`

```go
type Server struct {
    router         *mux.Router
    hybridBrain    *HybridBrain
    clClient       *cl_kernel.Client
    port           int
}

func NewServer(port int) *Server
func (s *Server) Start() error
func (s *Server) Stop() error
```

### 4.2 API Endpoints

| Method | Path | 描述 | 實現狀態 |
|--------|------|------|----------|
| POST | `/api/v1/execute` | 執行任務 | 待實現 |
| POST | `/api/v1/execute/stream` | 流式執行 | 待實現 |
| GET | `/api/v1/session/:id` | 獲取會話 | 待實現 |
| POST | `/api/v1/session/:id/message` | 發送消息 | 待實現 |
| GET | `/api/v1/budget` | 獲取預算 | 待實現 |
| POST | `/api/v1/budget/reset` | 重置預算 | 待實現 |
| GET | `/api/v1/stats` | 統計信息 | 待實現 |

### 4.3 WebSocket 支援

```go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // 升級為 WebSocket
    // 處理流式輸出
    // 管理連接生命週期
}
```

### 4.4 Middleware

```go
// 認證
func AuthMiddleware(next http.Handler) http.Handler

// 日誌
func LoggingMiddleware(next http.Handler) http.Handler

// CORS
func CORSMiddleware(next http.Handler) http.Handler
```

---

## 5. Phase 3: CLI 整合 (Day 9-11)

### 5.1 命令行參數

```bash
# 強制執行模式
crushcl --cl "task"           # 強制 CL 模式
crushcl --cc "task"          # 強制 CC 模式
crushcl --hybrid "task"      # 混合模式（默認）

# 預算控制
crushcl --budget 10.0 "task" # 設置預算上限

# API 模式
crushcl serve                # 啟動 HTTP Server
crushcl serve --port 8080   # 指定端口
```

### 5.2 配置檔案

```json
{
  "hybrid_brain": {
    "enabled": true,
    "default_mode": "auto",
    "budget": 10.0,
    "cl_threshold": "quick_lookup,file_operation",
    "cc_threshold": "github,complex_refactor,bug_hunt"
  },
  "api": {
    "enabled": true,
    "port": 8080,
    "auth": {
      "type": "api_key",
      "key": "${CRUSHCL_API_KEY}"
    }
  }
}
```

---

## 6. Phase 4: 測試與文檔 (Day 12-17)

### 6.1 Unit Tests

| 元件 | 測試內容 | 覆蓋目標 |
|------|----------|----------|
| TaskClassifier | 分類邏輯、邊界情況 | 90%+ |
| CostOptimizer | 成本估算、預算檢查 | 90%+ |
| ResourceManager | 併發安全、邊界情況 | 90%+ |
| TaskScheduler | 任務排程、優先級 | 90%+ |
| CLKernelClient | HTTP 調用、錯誤處理 | 80%+ |
| Server | API 端點、認證 | 80%+ |

### 6.2 集成測試

```bash
# 測試命令
go test ./internal/kernel/... -tags=integration

# 測試場景
1. 完整任務執行流程
2. 預算降級邏輯
3. CL/CC 切換
4. Session 恢復
5. 並發任務處理
```

### 6.3 文檔更新

| 文檔 | 更新內容 |
|------|----------|
| README.md | 新功能說明 |
| API.md | HTTP API 規範 |
| ARCHITECTURE.md | 更新架構圖 |
| 架構子目錄 | 補充各模組詳細設計 |

---

## 7. 實施順序

### 7.1 每日任務

| Day | Phase | 任務 |
|-----|-------|------|
| 0 | P0 | 緊急修復 |
| 1 | P1 | CL Kernel Client 框架 |
| 2 | P1 | CL Kernel Client 實現 |
| 3 | P1 | Task Classifier |
| 4 | P1 | Cost Optimizer |
| 5 | P1 | Task Scheduler + Resource Manager |
| 6 | P2 | HTTP Server 框架 |
| 7 | P2 | API Endpoints |
| 8 | P2 | WebSocket + Middleware |
| 9 | P3 | CLI 整合 |
| 10 | P3 | 配置支援 |
| 11 | P3 | CLI 測試 |
| 12-14 | P4 | Unit Tests |
| 15-17 | P4 | 集成測試 + 文檔 |

### 7.2 風險應對

| 風險 | 影響 | 緩解 |
|------|------|------|
| CL Kernel 介面不明確 | 高 | 先實現 mock，逐步完善 |
| 預算計算不準確 | 中 | 使用實際 API 成本數據 |
| 並發測試複雜 | 中 | 使用 goroutine leak detector |
| 文檔滯後 | 低 | 邊實現邊更新 |

---

## 8. 成功標準

### 8.1 功能標準

- [ ] TaskClassifier 準確率 > 80%
- [ ] CostOptimizer 誤差不超過 10%
- [ ] CL 模式響應延遲 < 500ms
- [ ] CC 模式正確傳遞工具調用
- [ ] 預算追蹤誤差不超過 1%

### 8.2 技術標準

- [ ] 所有核心模組 Unit Test 覆蓋 > 80%
- [ ] API 端點測試覆蓋 > 80%
- [ ] 無 goroutine leak
- [ ] 編譯無 warning
- [ ] `go vet` 無錯誤

### 8.3 文檔標準

- [ ] API.md 完整
- [ ] ARCHITECTURE.md 更新
- [ ] 各模組有 README
- [ ] 有使用範例

---

## 9. 資源估算

### 9.1 程式碼量

| 元件 | 預估行數 |
|------|----------|
| CL Kernel Client | 500 |
| Task Scheduler | 300 |
| Resource Manager | 200 |
| Load Balancer | 200 |
| Task Classifier | 300 |
| Cost Optimizer | 200 |
| HTTP Server | 500 |
| CLI 整合 | 300 |
| Tests | 1500 |
| **總計** | ~4000 |

### 9.2 依賴

```
go get github.com/gorilla/mux
go get github.com/gorilla/websocket
go get github.com/stretchr/testify  # testing
```

---

## 10. 維護建議

### 10.1 Code Review Checklist

- [ ] 所有 PR 需通過 `go build`
- [ ] 所有 PR 需通過 `go vet`
- [ ] 新功能需有對應測試
- [ ] 併發程式需通過 race detector

### 10.2 Monitoring

```bash
# 啟動時添加
go run -race ./cmd/crushcl
```

### 10.3 Performance Budget

- CLI 啟動時間 < 500ms
- Session 恢復 < 1s
- 併發任務支持 10+

---

*計劃版本: 1.0 | 最後更新: 2026-04-04*
