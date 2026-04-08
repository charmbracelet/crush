# Claude Code 架構使用指南
## How to Use the Integrated Claude Code Architecture

**日期**: 2026-04-03  
**專案**: crush-magical

---

## 目錄

1. [整合狀態](#1-整合狀態)
2. [Context Compactor 使用方式](#2-context-compactor-使用方式)
3. [Memory Store 使用方式](#3-memory-store-使用方式)
4. [Tool Registry 使用方式](#4-tool-registry-使用方式)
5. [Permission Grading 使用方式](#5-permission-grading-使用方式)
6. [在 Agent 中使用](#6-在-agent-中使用)

---

## 1. 整合狀態

所有核心組件已整合到 `internal/agent/agent.go` 的 `sessionAgent` 結構中：

```go
type sessionAgent struct {
    // ... 其他欄位 ...
    
    // Kernel components: Claude Code architectural patterns
    compactor    *kctx.ContextCompactor
    memStore     *memory.MemoryStore
    toolRegistry *registry.ToolRegistry
}
```

### 初始化位置

組件在 `NewSessionAgent()` 中自動初始化 (`agent.go:170-175`)：

```go
// Initialize kernel components (Claude Code patterns)
compactor:    kctx.NewContextCompactor(200000),
memStore:     memory.NewMemoryStore(""),
toolRegistry: registry.NewToolRegistry(),
```

---

## 2. Context Compactor 使用方式

### 2.1 基本 API

```go
import kcontext "github.com/charmbracelet/crush/internal/kernel/context"

// 創建 compactor (已由 agent 自動初始化)
compactor := kctx.NewContextCompactor(200000) // 200000 tokens

// 檢查是否需要自動壓縮
if compactor.ShouldAutoCompact(currentTokenCount) {
    // 觸發壓縮邏輯
}

// 記錄工具結果以供後續壓縮
compactor.RecordToolResult(toolID, toolName, output)

// 檢查工具是否可壓縮
if compactor.IsCompactable("Read") {
    // 處理可壓縮的工具
}

// 執行快速微壓縮 (<1ms)
messages = compactor.Microcompact(messages)

// 獲取壓縮指標
metrics := compactor.Metrics()
```

### 2.2 4層壓縮級別

```go
// 等級定義
const (
    Microcompact         CompressionLevel = iota + 1  // <1ms, 基於規則
    AutoCompact                                      // 閾值觸發
    FullCompact                                      // Fork agent 摘要, 5-30s
    SessionMemoryCompact                             // 使用現有摘要, <10ms
)

// 使用範例
switch compactor.GetCompressionLevel() {
case kctx.Microcompact:
    // 快速清理
case kctx.AutoCompact:
    // 使用 LLM 或 SM 壓縮
case kctx.FullCompact:
    // Fork 子 agent 進行摘要
case kctx.SessionMemoryCompact:
    // 使用現有摘要快速恢復
}
```

### 2.3 可壓縮工具清單

```go
// 預設可壓縮工具
const (
    ToolRead     CompactableTool = "Read"
    ToolBash     CompactableTool = "Bash"
    ToolGrep     CompactableTool = "Grep"
    ToolGlob     CompactableTool = "Glob"
    ToolWebFetch CompactableTool = "WebFetch"
    ToolEdit     CompactableTool = "Edit"
    ToolWrite    CompactableTool = "Write"
)
```

---

## 3. Memory Store 使用方式

### 3.1 基本 API

```go
import "github.com/charmbracelet/crush/internal/kernel/memory"

// 創建 memory store (已由 agent 自動初始化)
memStore := memory.NewMemoryStore(dataDir)

// 添加會話記憶 (Layer 1 - 臨時)
entry := memory.MemoryEntry{
    ID:          "mem-1",
    Type:        memory.MemoryTypeUser,
    Name:        "User Preference",
    Description: "User prefers dark mode",
    Content:     "User has set dark mode as default theme",
    Why:         "Important user preference discovered",
    HowToApply:  "Apply dark mode by default in UI settings",
}
memStore.AddSessionMemory(entry)

// 添加持久記憶 (Layer 2 - 永久)
memStore.AddPersistentMemory(entry)

// 添加團隊記憶 (Layer 3 - 跨用戶)
memStore.AddTeamMemory(entry)

// 檢索記憶 (按相關性排序)
memories := memStore.GetMemory(memory.MemoryTypeUser, 10)

// 按 Weibull 衰減後的相關性排序檢索
relevantMemories := memStore.GetMemoryWithDecay(memory.MemoryTypeUser, 10)

// 獲取上下文摘要 (用於 system prompt)
contextSummary := memStore.GetContextSummary()

// 清除會話記憶
memStore.ClearSession()
```

### 3.2 Weibull 衰減配置

```go
// 預設配置
config := memory.DefaultWeibullConfig()
// Shape = 0.8, Scale = 168 小時 (1週)

// 自定義配置
config := memory.WeibullDecayConfig{
    Shape: 0.8,  // 形狀參數: <1 早期衰減快, =1 指數衰減, >1 延遲衰減
    Scale: 168,  // 尺度參數: 特徵時間 (小時)
}

// 計算衰減值 (0-1, 1=完全記憶, 0=已遺忘)
decay := config.CalculateDecay(ageHours)

// 計算記憶相關性
relevance := memStore.CalculateMemoryRelevance(entry)
```

### 3.3 Weibull 衰減計算公式

```
S(t) = exp(-(t/λ)^k)

其中:
- t = 年齡 (小時)
- λ (Scale) = 168 (特徵時間, 1週)
- k (Shape) = 0.8 (形狀參數)

衰減範例:
- 0 小時: 1.0000 (全新記憶)
- 24 小時: 0.8099 (1天後)
- 168 小時: 0.3679 (1週後)
- 720 小時: 0.0406 (1月後)
```

---

## 4. Tool Registry 使用方式

### 4.1 基本 API

```go
import "github.com/charmbracelet/crush/internal/kernel/registry"

// 創建 registry (已由 agent 自動初始化)
toolReg := registry.NewToolRegistry()

// 註冊工具
toolReg.Register(&registry.ToolMetadata{
    Name:         "Read",
    Aliases:      []string{"view", "cat"},
    Description:  "Read file contents",
    Capabilities: []registry.ToolCapability{
        registry.CapabilityRead,
        registry.CapabilityFileSystem,
    },
    Version:      "1.0",
    AutoDiscover: true,
})

// 直接查詢
meta, ok := toolReg.Get("Read")
if ok {
    fmt.Println(meta.Name)        // "Read"
    fmt.Println(meta.Aliases)    // [view cat]
}

// 別名解析
meta, ok = toolReg.Get("view")  // 自動解析為 "Read"
```

### 4.2 能力查詢

```go
// 查詢具有特定能力的工具
readTools := toolReg.FindByCapability(registry.CapabilityRead)
for _, tool := range readTools {
    fmt.Println(tool.Name)
}

// 獲取所有能力類型
capabilities := toolReg.GetCapabilities()

// 列出所有已註冊工具
allTools := toolReg.List()
```

### 4.3 預設能力類型

```go
const (
    CapabilityRead       ToolCapability = "read"
    CapabilityWrite      ToolCapability = "write"
    CapabilityExecute    ToolCapability = "execute"
    CapabilitySearch     ToolCapability = "search"
    CapabilityAnalysis   ToolCapability = "analysis"
    CapabilityNetwork    ToolCapability = "network"
    CapabilityFileSystem ToolCapability = "filesystem"
)
```

### 4.4 模式發現

```go
// 預設發現模式
patterns := []DiscoveryPattern{
    {Name: "read_pattern", Matcher: func(name string) bool {
        return containsAny(name, "read", "view", "cat", "fetch", "get")
    }},
    {Name: "write_pattern", Matcher: func(name string) bool {
        return containsAny(name, "write", "edit", "create", "save", "put")
    }},
    {Name: "execute_pattern", Matcher: func(name string) bool {
        return containsAny(name, "run", "exec", "bash", "shell", "command")
    }},
    {Name: "search_pattern", Matcher: func(name string) bool {
        return containsAny(name, "search", "grep", "find", "query", "ls")
    }},
    {Name: "network_pattern", Matcher: func(name string) bool {
        return containsAny(name, "http", "fetch", "web", "url", "request")
    }},
}

// 使用模式發現工具
tools := toolReg.FindByPattern("read_pattern")

// 自動發現新工具
discovered := toolReg.Discover([]string{"ToolName1", "ToolName2"})
```

---

## 5. Permission Grading 使用方式

### 5.1 基本 API

```go
import "github.com/charmbracelet/crush/internal/kernel/permission"

// 獲取工具/動作的權限級別
grade := permission.GetGrade("Bash", "execute")

// 檢查級別
if grade.Level == permission.LevelAdmin {
    // 高風險操作，需要確認
}

// 檢查是否需要用戶批准
if grade.RequiresApproval() {
    // 請求用戶批准
}

// 檢查是否高風險
if grade.IsHighRisk() {
    // 記錄審計日誌
}
```

### 5.2 3級權限系統

```go
// 權限級別定義
const (
    LevelRead  PermissionLevel = iota + 1  // L1: 唯讀操作
    LevelWrite                              // L2: 寫入操作
    LevelAdmin                              // L3: 高風險操作
)

// 級別說明
grade := permission.GetGrade("Bash", "execute")
// grade.Level = LevelAdmin
// grade.Name = "Admin"
// grade.RiskLevel = "high"
// grade.Description = "High-risk operations..."
```

### 5.3 工具分類規則

```go
// L1 Read - 唯讀工具
readTools := ["view", "read", "cat", "grep", "find", "search", 
              "ls", "dir", "glob", "stat", "file", "head", "tail",
              "wc", "diff", "inspect", "fetch", "url", "webfetch"]

// L2 Write - 寫入工具
writeTools := ["write", "edit", "create", "save", "mkdir", 
                "touch", "append", "patch", "replace"]

// L3 Admin - 高風險工具
adminTools := ["bash", "shell", "exec", "run", "sudo", "chmod",
                "chown", "system", "config", "network", "ssh",
                "scp", "curl", "wget", "request", "http",
                "postgres", "mysql", "redis", "mongo", "docker",
                "kubectl", "git"]

// 高風險動作 (任何工具 + 這些動作 = L3)
destructiveActions := ["delete", "remove", "rm", "destroy", "drop",
                       "truncate", "format", "erase", "kill",
                       "terminate", "stop", "shutdown", "reboot", "halt"]
```

### 5.4 權限檢查流程

```go
func CheckPermission(toolName, action string) (bool, error) {
    grade := permission.GetGrade(toolName, action)
    
    switch grade.Level {
    case permission.LevelRead:
        // L1: 自動批准
        return true, nil
        
    case permission.LevelWrite:
        // L2: 檢查允許清單
        if isInAllowList(toolName, action) {
            return true, nil
        }
        // 否則請求用戶批准
        return requestApproval(grade)
        
    case permission.LevelAdmin:
        // L3: 總是請求用戶批准
        return requestApproval(grade)
    }
    
    return false, errors.New("unknown permission level")
}
```

---

## 6. 在 Agent 中使用

### 6.1 自動初始化

所有組件已在 `NewSessionAgent()` 中自動初始化：

```go
func NewSessionAgent(opts SessionAgentOptions) SessionAgent {
    return &sessionAgent{
        // ... 其他初始化 ...
        
        // Kernel components: Claude Code architectural patterns
        compactor:    kctx.NewContextCompactor(200000),
        memStore:     memory.NewMemoryStore(""),
        toolRegistry: registry.NewToolRegistry(),
    }
}
```

### 6.2 在 Run 回調中使用

```go
func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
    // ... 前置邏輯 ...
    
    // 在 OnToolResult 回調中使用 compactor
    OnToolResult: func(result fantasy.ToolResultContent) error {
        // 記錄工具結果以供壓縮
        a.compactor.RecordToolResult(result.ToolCallID, result.ToolName, "")
        
        // 在 Memory Store 中記錄記憶
        if isImportantResult(result) {
            a.memStore.AddSessionMemory(memory.MemoryEntry{
                ID:      result.ToolCallID,
                Name:    result.ToolName,
                Content: extractContent(result),
            })
        }
        
        return nil
    }
    
    // 在 OnStepFinish 回調中使用 compactor
    OnStepFinish: func(stepResult fantasy.StepResult) error {
        // 檢查是否需要觸發壓縮
        tokens := stepResult.Usage.InputTokens + stepResult.Usage.OutputTokens
        if a.compactor.ShouldAutoCompact(tokens) {
            // 觸發自動壓縮
            a.compactor.Microcompact(preparedMessages)
        }
        return nil
    }
}
```

### 6.3 完整使用範例

```go
package main

import (
    "fmt"
    
    kcontext "github.com/charmbracelet/crush/internal/kernel/context"
    "github.com/charmbracelet/crush/internal/kernel/memory"
    "github.com/charmbracelet/crush/internal/kernel/permission"
    "github.com/charmbracelet/crush/internal/kernel/registry"
)

func main() {
    // 初始化組件
    compactor := kcontext.NewContextCompactor(200000)
    memStore := memory.NewMemoryStore("./data")
    toolReg := registry.NewToolRegistry()
    
    // 1. 註冊工具
    toolReg.Register(&registry.ToolMetadata{
        Name:         "Read",
        Capabilities: []registry.ToolCapability{registry.CapabilityRead},
    })
    
    // 2. 檢查權限
    grade := permission.GetGrade("Read", "read")
    fmt.Printf("Permission level: %s (risk: %s)\n", grade.Name, grade.RiskLevel)
    
    // 3. 記錄工具結果
    compactor.RecordToolResult("tool-1", "Read", "file content...")
    
    // 4. 檢查是否需要壓縮
    if compactor.ShouldAutoCompact(180000) {
        fmt.Println("Triggering context compaction...")
    }
    
    // 5. 保存記憶
    memStore.AddSessionMemory(memory.MemoryEntry{
        ID:      "mem-1",
        Type:    memory.MemoryTypeUser,
        Name:    "Preference",
        Content: "User prefers concise responses",
    })
    
    // 6. 獲取上下文摘要
    summary := memStore.GetContextSummary()
    fmt.Printf("Context summary:\n%s\n", summary)
}
```

---

## 7. 測試指令

```bash
# 運行整合測試報告
cd G:/AI分析/crush-magical
go run ./internal/kernel/test_report.go

# 運行單元測試
go test ./internal/kernel/... -v

# 運行權限測試
go test ./internal/kernel/permission/... -v

# 運行所有測試
go test ./...

# 編譯檢查
go build ./...
```

---

## 8. 檔案位置

| 檔案 | 功能 |
|------|------|
| `internal/kernel/context/compactor.go` | 4層上下文壓縮 |
| `internal/kernel/memory/store.go` | 3層記憶體 + Weibull |
| `internal/kernel/registry/registry.go` | 動態工具發現 |
| `internal/kernel/permission/grade.go` | 3級權限分級 |
| `internal/agent/agent.go` | Agent 整合 |

---

*使用指南生成時間: 2026-04-03*
