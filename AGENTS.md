# CrushCL Agent 行為指南

## 項目定位

```
┌─────────────────────────────────────────────────────────────┐
│                        CrushCL                               │
│                                                              │
│  官方 Crush (upstream)  ←───  溝通/分析  ───→  各大 AI Agent │
│         │                                                    │
│         │ 同步官方                                            │
│         ▼                                                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              設計/協調/介面標準化                       │   │
│  └─────────────────────────────────────────────────────┘   │
│         │                                                    │
│         │ 傳遞設計意圖                                        │
│         ▼                                                    │
│  CrushCL (Executor) ──→ 實現各 AI Agent 逆向工程的技術結晶   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 角色分工

| 角色 | 負責範圍 |
|------|---------|
| **Architect (我)** | 官方 Crush 協調、跨 Agent 溝通、架構設計 |
| **Executor (crushcl)** | 根據設計進行具體實現、測試驗證 |

## 核心原則

### 跟隨官方 Crush

- CrushCL 核心架構 → **必須**與官方 Crush 保持同步
- Claude Code 啟發 → **可選**增強層，可插拔
- 避免直接複製 Claude Code 邏輯，與官方 Crush 結構脫節

### 介面標準化

```go
// 標準化 Compression interface
type Compressor interface {
    Compact(ctx context.Context, messages []Message) ([]Message, error)
}

// 官方實現
type OfficialCompressor struct{}

// Claude Code 啟發實現
type ClaudeInspiredCompressor struct{}

// 根據配置選擇
func NewCompressor(cfg Config) Compressor {
    if cfg.UseClaudeInspired {
        return &ClaudeInspiredCompressor{}
    }
    return &OfficialCompressor{}
}
```

## Claude Code 4 層壓縮系統

### 觸發閾值

| 層級 | 名稱 | 觸發條件 | 延遲 |
|------|------|---------|------|
| L1 | 微壓縮 | 每輪結束 + >20 tool results | <1ms |
| L2 | 自動壓縮 | Token ≥ 85% budget (170K/200K) | ~100ms |
| L3 | 完整壓縮 | Token ≥ 95% budget (190K/200K) | 5-30s |
| L4 | 會話記憶 | Token ≥ 85% + existing collapses | <10ms |

### Token 預算

```
Max Token Budget: 200000
├── L1: < 85% (0-170K) + tool count > 20
├── L2: 85-95% (170K-190K)
├── L3: ≥ 95% (≥190K)
└── L4: ≥ 85% + has collapses
```

## 協作流程

```
1. 官方 Crush 更新 → Architect 分析變更，評估影響
2. Claude Code 新功能 → Architect 逆向分析，提取模式
3. 設計整合方案 → Architect 輸出介面規格和實現指引
4. CrushCL 實現 → Executor 根據設計進行實現
5. 問題反饋 → Executor 提出實現中的問題，Architect 調整設計
```

## 專案結構

```
/crushcl
  /internal
    /core          ← 跟隨官方 Crush 架構
      /agent
      /session
      /message
    /kernel        ← Claude Code 啟發（可選/可插拔）
      /context
      /loop
      /memory
      /permission
      /registry
  /AGENTS.md      ← 本文件
```

## 編碼原則

1. **核心優先** - 先確保與官方 Crush 同步，Claude Code 啟發作為增強
2. **介面抽象** - 使用介面隔離實現，便於替換
3. **可插拔設計** - Claude Code 特性應可通過配置開關
4. **測試覆蓋** - 新功能必須有對應測試

## 壓縮系統文件

| 檔案 | 職責 |
|------|------|
| `kernel/context/kernel_context.go` | 原版 ContextManager |
| `kernel/context/compactor.go` | 4-tier 壓縮觸發器 + SM Compression |
| `kernel/context/session_memory_pool.go` | SessionMemoryPool - SM 記憶體區塊池 |
| `kernel/context/memory_hit_calculator.go` | MemoryHitCalculator - 覆蓋率計算 |
| `kernel/context/sm_composer.go` | SMComposer - 摘要文本構建 |
| `kernel/hook_pipeline.go` | HookPipeline - PreToolUse/PostToolUse 管道 |
| `kernel/usage_tracker.go` | EnhancedUsageTracker - 用量追蹤 |
| `kernel/compression_orchestrator.go` | CompressionOrchestrator - 層級協調器 |
| `kernel/loop/loop.go` | 狀態機定義 |
| `agent/agent.go` | PrepareStep 整合 |

## 壓縮架構總覽

```
CompressionOrchestrator
    │
    ├── HookPipeline
    │   ├── PreToolUse hooks
    │   ├── PostToolUse hooks
    │   ├── PreCompact hooks
    │   └── PostCompact hooks
    │
    ├── EnhancedUsageTracker
    │   ├── Token tracking
    │   ├── Cost tracking
    │   └── Budget management
    │
    └── ContextCompactor
        ├── L1Microcompact (<1ms)
        ├── L2AutoCompact (~100ms)
        ├── L3FullCompact (5-30s)
        └── L4SessionMemory (<10ms)
            ├── SessionMemoryPool
            ├── MemoryHitCalculator
            └── SMComposer
```

## 禁止事項

| ❌ 禁止 | ✅ 正確做法 |
|--------|-----------|
| 直接複製 Claude Code 邏輯 | 借鑒思想，獨立實現 |
| 追逐 Claude Code 版本號 | 專注 CrushCL 自己版本 |
| 與官方 Crush 結構綁定過深 | 抽象化介面 |

## 上下文管理

### 監控閾值

| 等級 | 閾值 | 行動 |
|------|------|------|
| 正常 | < 50k tokens | 無需行動 |
| 注意 | 50k - 80k | 開始增量摘要 |
| **安全寫入點** | ~85k | **立即寫入 worklog** |
| 警告 | 80k - 120k | 確保日誌已寫入 |
| 危險 | > 120k | 暫停新任務 |

---
*CrushCL Agent 行為指南*
*建立時間: 2026-04-03*

## 記住的資訊

| 項目 | 值 | 記錄時間 |
|------|------|---------|
| 使用者數字 | 12345 | 2026-04-03 |
