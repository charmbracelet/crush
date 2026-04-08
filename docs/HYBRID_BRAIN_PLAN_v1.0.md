# CrushCL Claude Code Bridge - 完善規劃文檔

**版本**: 1.0  
**日期**: 2026-04-03  
**狀態**: 規劃中

---

## 1. 願景與目標

### 1.1 最終目標
建立一個智能混合代理系統，由 Crush (本地大腦) 統一調度本地 CLI (CL) 和 Claude Code (CC)，根據任務複雜度、預算和性能自動選擇最優執行路徑。

### 1.2 核心價值
- **成本優化**: 簡單任務用 CL (成本接近零)，複雜任務才調用 CC
- **性能優先**: 減少網路延遲，本地處理優先
- **智能路由**: 自動分類任務，選擇最佳執行者
- **資源整合**: 統一管理兩套工具生態系統

---

## 2. 架構設計

### 2.1 系統架構圖

```
┌─────────────────────────────────────────────────────────────────┐
│                        Crush (大腦)                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐    │
│  │ TaskClassifier│  │ CostOptimizer│  │ ExecutorSelector │    │
│  └──────────────┘  └──────────────┘  └──────────────────┘    │
│           │               │                    │                │
│           └───────────────┼────────────────────┘                │
│                           ▼                                     │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    HybridBrain                            │  │
│  │  ┌─────────────────┐    ┌─────────────────────────────┐  │  │
│  │  │   CL Native     │    │     Claude Code CLI         │  │  │
│  │  │   Executor      │    │     Executor               │  │  │
│  │  └─────────────────┘    └─────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
           │                                  │
           ▼                                  ▼
┌─────────────────────┐          ┌─────────────────────────────┐
│   CrushCL Kernel    │          │     Claude Code CLI          │
│   - Fantasy Agent   │          │     - claude -p "..."       │
│   - Tool Registry   │          │     - JSON Output           │
│   - Session Mgmt   │          │     - Tool Integration      │
│   - MCP Tools      │          │                             │
└─────────────────────┘          └─────────────────────────────┘
```

### 2.2 核心元件

| 元件 |職責 | 狀態 |
|------|------|------|
| HybridBrain | 任務分類、路由、執行協調 | ⚠️ 需完善 |
| CLKernelClient | CrushCL 內核調用 | ⚠️ 需完善 |
| CCBridge | Claude Code CLI 橋接 | ✅ 已實現 |
| SessionManager | 多輪對話歷史 | ✅ 已實現 |
| BudgetManager | 跨會話預算追蹤 | ✅ 已實現 |
| TaskClassifier | 任務意圖分類 | ✅ 已實現 |
| CostOptimizer | 成本估算與決策 | ✅ 已實現 |

---

## 3. 執行模式

### 3.1 任務分類

| TaskType | 描述 | 執行者 | 成本估算 |
|----------|------|--------|---------|
| TaskQuickLookup | 快速查詢、定義、解釋 | CL | $0.001 |
| TaskFileOperation | 檔案讀寫、編輯 | CL | $0.002 |
| TaskDataProcessing | 數據處理、轉換 | CL | $0.003 |
| TaskGitHub | PR、Commit、Branch | CC | $0.05 |
| TaskComplexRefactor | 複雜重構、重構 | CC | $0.10 |
| TaskCodeReview | 代碼審查、安全掃描 | CC | $0.05 |
| TaskBugHunt | Bug 追蹤、調試 | CC | $0.08 |
| TaskCreative | 創意任務、頭腦風暴 | CC | $0.03 |
| TaskMCPTask | MCP 工具任務 | CC | $0.05 |

### 3.2 執行流程

```
用戶輸入
    │
    ▼
HybridBrain.Think()
    │
    ├── TaskClassifier.Classify() → TaskType
    │
    ├── CostOptimizer.Estimate() → CostUSD
    │
    ├── 預算檢查
    │   └── 預算不足 → 降級到 CL
    │
    ├── 執行路由
    │   ├── CL: executeViaCL() → CrushCL Kernel
    │   ├── CC: executeViaClaudeCode() → Claude Code CLI
    │   └── Hybrid: 兩者協作
    │
    └── 結果整合 → 返回給用戶
```

---

## 4. 待實現功能

### 4.1 緊急 (P0)

#### 4.1.1 CL Kernel 整合
```
需求:
- 實現真正的 CrushCL 內核調用
- 通過 HTTP/gRPC 與 CrushCL 服務通信
- 支援流式輸出

接口定義:
type CLKernelClient interface {
    Execute(ctx context.Context, prompt string, tools []string) (*ExecutionResult, error)
    ExecuteStream(ctx context.Context, prompt string, tools []string, stream func(string) error) error
    GetSession() (*SessionState, error)
    UpdateSession(sessionID string) error
}
```

#### 4.1.2 Coordination 模組
```
需求:
- 任務調度器
- 資源管理器
- 負載均衡

目錄: kernel/coordination/
- task_scheduler.go: 任務隊列、優先級調度
- resource_manager.go: 併發控制、資源追蹤
- load_balancer.go: CL/CC 負載均衡
```

#### 4.1.3 HTTP API Server
```
需求:
- 暴露 kernel 能力供外部調用
- RESTful API
- WebSocket 支援流式輸出

端點:
- POST /api/v1/execute - 執行任務
- GET /api/v1/session/:id - 獲取會話
- POST /api/v1/session/:id/message - 發送消息
- GET /api/v1/budget - 獲取預算狀態
- POST /api/v1/budget/reset - 重置預算
```

### 4.2 重要 (P1)

#### 4.2.1 MCP Server 實現
```
需求:
- 實現 Model Context Protocol 服務端
- 支援標準 MCP 工具
- 與 CrushCL 工具生態整合

目錄: kernel/mcp/
- mcp_server.go: 主服務器
- mcp_handlers.go: 工具處理器
- mcp_transport.go: 傳輸層 (stdio, HTTP)
```

#### 4.2.2 CLI 整合
```
需求:
- 將 HybridBrain 整合到 CrushCL 主 CLI
- 支持 --cl, --cc, --hybrid 模式
- 配置文件支持

命令:
- crushcl --cl "task" - 強制 CL 模式
- crushcl --cc "task" - 強制 CC 模式
- crushcl --budget $10 - 設置預算
```

#### 4.2.3 會話持久化
```
需求:
- SQLite 會話存儲
- 跨進程會話恢復
- 會話歷史導出

表結構:
- sessions: id, created_at, updated_at, title, cost, tokens
- messages: id, session_id, role, content, timestamp
- tasks: id, session_id, task_type, executor, cost, duration
```

### 4.3 增強 (P2)

#### 4.3.1 監控儀表板
```
功能:
- 預算使用實時監控
- 任務分佈統計
- 成本趨勢分析
```

#### 4.3.2 插件系統
```
功能:
- 自定義任務分類器
- 自定義執行者
- 自定義工具
```

---

## 5. 數據模型

### 5.1 核心類型

```go
// 任務分類
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

// 執行結果
type ExecutionResult struct {
    Output      string
    Executor    ExecutorType
    Tokens      int
    Cost        float64
    Duration    time.Duration
    Error       error
    CacheHit    bool
}

// 執行者類型
type ExecutorType string
const (
    ExecutorCL    ExecutorType = "cl"
    ExecutorCC    ExecutorType = "claudecode"
    ExecutorHybrid ExecutorType = "hybrid"
)

// 會話狀態
type SessionState struct {
    ID           string
    CreatedAt    time.Time
    Messages     []Message
    TaskHistory  []TaskRecord
    TotalCostUSD float64
    TotalTokens  int
}

// 任務記錄
type TaskRecord struct {
    Task          string
    Classification TaskClassification
    Result        ExecutionResult
    Timestamp     time.Time
}
```

---

## 6. API 規範

### 6.1 Execute

```
POST /api/v1/execute

Request:
{
    "prompt": "string",
    "tools": ["Read", "Write", "Bash"],
    "executor": "auto|cl|cc|hybrid",
    "model": "sonnet|opus|haiku",
    "stream": false
}

Response:
{
    "session_id": "string",
    "text": "string",
    "tokens": 1000,
    "cost_usd": 0.05,
    "executor": "claudecode",
    "duration_ms": 5000
}
```

### 6.2 Stream Execute

```
POST /api/v1/execute/stream

Request: 同上

Response: Server-Sent Events
data: {"type": "chunk", "content": "He"}
data: {"type": "chunk", "content": "llo"}
data: {"type": "done", "tokens": 2}
```

---

## 7. 測試策略

### 7.1 單元測試
- TaskClassifier 分類邏輯
- CostOptimizer 成本估算
- SessionManager 會話管理
- BudgetManager 預算追蹤

### 7.2 集成測試
- CL Kernel 調用
- Claude Code Bridge
- HTTP API 端點

### 7.3 端到端測試
- 完整工作流
- 預算降級
- 錯誤處理

---

## 8. 實施計劃

### Phase 1: 核心完善 (1-2 週)
1. 實現 CL Kernel Client
2. 實現 Coordination 模組
3. 實現 HTTP API Server
4. CLI 整合

### Phase 2: 高級功能 (2-3 週)
1. MCP Server 實現
2. 會話持久化
3. 流式輸出支援

### Phase 3: 增強功能 (1-2 週)
1. 監控儀表板
2. 插件系統
3. 性能優化

---

## 9. 風險與依賴

### 9.1 技術風險
| 風險 | 影響 | 緩解 |
|------|------|------|
| Claude Code CLI 不穩定 | 高 | 添加超時、重試機制 |
| 預算計算不準確 | 中 | 使用實際 API 成本 |
| 會話狀態丟失 | 中 | 添加持久化層 |

### 9.2 依賴
- CrushCL 核心 (fantasy, agent)
- Claude Code CLI (claude command)
- Go 1.21+

---

## 10. 成功標準

### 10.1 功能標準
- [ ] 80%+ 任務正確分類
- [ ] CL 模式處理簡單任務延遲 < 100ms
- [ ] CC 模式正確傳遞工具調用
- [ ] 預算追蹤誤差 < 1%

### 10.2 性能標準
- [ ] CLI 啟動時間 < 500ms
- [ ] 會話恢復時間 < 1s
- [ ] 並發任務支持 10+

---

*文檔版本: 1.0 | 最後更新: 2026-04-03*
