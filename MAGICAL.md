# Crush-Magical: Enhanced AI Agent

## 概述

Crush-Magical 是基於 Crush 的魔改版本，整合了 `kernel/` 目錄中原本未被使用的 4 個核心模組，實現更高效的 AI Agent 系統。

## 整合的 Kernel 模組

### 1. kernel/memory - Weibull 衰減記憶系統

**位置**: `internal/agent/kernel_memory.go`

**功能**:
- 3 層記憶架構：Peripheral → Working → Core
- Weibull 衰減演算法：`exp(-(age/λ)^k)`
- 自動晉升/降級重要記憶
- 跨 session 持久化

**使用**:
```go
// 記錄工具調用
a.kernelMemory.AddToolCall("read", "file_path")

// 記錄用戶消息
a.kernelMemory.AddUserMessage("幫我分析這個項目")

// 獲取最近工具調用
recentTools := a.kernelMemory.GetRecentToolCalls(10)

// 獲取記憶狀態摘要
summary := a.kernelMemory.GetContextSummary()
```

### 2. kernel/loop - 狀態機迴圈檢測

**位置**: `internal/agent/kernel_loop.go`

**功能**:
- 工具調用簽名追蹤
- 熔斷器（Circuit Breaker）模式
- Error Withholding 錯誤緩衝
- 自動迴圈檢測（3次重複觸發）

**使用**:
```go
// 記錄工具調用並檢測迴圈
if err := a.kernelLoop.RecordToolCall("read", "file"); err != nil {
    return err // 迴圈被檢測到
}

// 檢查熔斷器狀態
if !a.kernelLoop.AllowRequest() {
    return errors.New("circuit breaker open")
}

// 獲取當前輪次
turn := a.kernelLoop.GetTurnCount()
```

### 3. kernel/coordination - 多 Agent 協作

**位置**: `internal/agent/kernel_coordinator.go`

**功能**:
- Mailbox 異步消息隊列
- Phase 驅動工作流：Research → Synthesis → Implementation → Verification
- PermissionBridge 權限控制
- 任務分派與結果聚合

**使用**:
```go
// 註冊子 Agent
a.kernelCoordinator.RegisterSubAgent("worker-1")

// 發送任務
a.kernelCoordinator.SendTaskAsync("worker-1", map[string]interface{}{
    "task": "analyze_code",
    "path": "/path/to/code",
})

// 執行完整工作流
phase, err := a.kernelCoordinator.ExecutePhaseWorkflow(ctx)
```

### 4. kernel/context - 壓縮管理

**位置**: `internal/agent/kernel_context.go`

**功能**:
- ToolBudget 工具預算管理
- CollapseCommit 對話摘要
- 8 級壓縮策略
- 熔斷追蹤

**使用**:
```go
// 添加工具結果
a.kernelContext.AddToolResult(id, "read", input, output)

// 凍結重要結果
a.kernelContext.FreezeResult(id)

// 檢查是否需要自動壓縮
if a.kernelContext.ShouldAutoCompact(0.85) {
    // 執行壓縮
}

// 應用壓縮策略
messages = a.kernelContext.CompressContext(messages, CompressCollapse)
```

## 架構整合

### sessionAgent 結構

```go
type sessionAgent struct {
    // ... 現有欄位 ...
    
    // Kernel 適配器
    kernelMemory      *kernelMemoryAdapter
    kernelLoop        *turnLoopAdapter
    kernelCoordinator *kernelCoordinatorAdapter
    kernelContext     *kernelContextAdapter
}
```

### 執行流程

```
用戶請求
    ↓
[1] 熔斷器檢查 (kernelLoop)
    ↓
[2] Turn 計數 + 1
    ↓
[3] 記錄用戶消息 (kernelMemory)
    ↓
[4] Agent 執行
    ├── [4a] 工具調用 → 迴圈檢測 + 記憶記錄
    ├── [4b] 工具結果 → 上下文預算
    └── [4c] 重試 → 錯誤記錄 + 熔斷追蹤
    ↓
[5] 成功 → 記憶保存 + 循環關閉
    ↓
返回結果
```

## 檔案變更

### 新增檔案

| 檔案 | 描述 |
|------|------|
| `internal/agent/kernel_memory.go` | Weibull 衰減記憶適配器 |
| `internal/agent/kernel_loop.go` | 狀態機迴圈檢測適配器 |
| `internal/agent/kernel_coordinator.go` | 多 Agent 協調適配器 |
| `internal/agent/kernel_context.go` | 壓縮管理適配器 |

### 修改檔案

| 檔案 | 修改內容 |
|------|----------|
| `internal/agent/agent.go` | 整合所有 kernel 適配器 |
| `go.mod` | 更新 module name |

## 編譯

```bash
cd G:/AI分析/crush-magical
go mod tidy
go build -o crush-magical ./main.go
```

## 測試

```bash
go test ./internal/agent/...
```

## 與原版 Crush 差異

| 功能 | 原版 Crush | Crush-Magical |
|------|-----------|---------------|
| 記憶管理 | 簡單分層 | Weibull 衰減 + 自動晉升 |
| 迴圈檢測 | 簡單計數 | 簽名追蹤 + 熔斷器 |
| 錯誤處理 | 直接失敗 | Error Withholding 緩衝 |
| 多 Agent | swarm (基礎) | Phase 工作流 + Mailbox |
| 上下文 | 固定窗口 | ToolBudget + Collapse |

## 下一步優化方向

1. **持久化 kernelMemory** 到磁碟，支援跨重啟記憶
2. **動態 Phase 工作流**，根據任務類型自定義階段
3. **分散式 Agent**，透過 Mailbox 跨進程通信
4. **自適應壓縮**，根據 token 使用率動態調整策略
