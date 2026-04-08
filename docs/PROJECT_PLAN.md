# CrushCL 多智能體協作系統 - 項目文檔

## 專案概述

CrushCL 是一個基於 Go 語言的多智能體協作框架，借鑒 Claude Code 架構思想，實現真正的 Agent 協作系統。

### 核心目標
- 支援 Windows 平台
- 實現真正的 Agent 協作（非簡單調用）
- 基於任務依賴的智能調度
- 可擴展的架構設計

---

## 目錄結構

```
G:/AI分析/crushcl/
├── internal/
│   ├── agent/
│   │   ├── dependency/          # 任務依賴管理系統 (已完成)
│   │   │   ├── dependency.go   # 主實現 (754行)
│   │   │   ├── dependency_test.go # 測試 (638行)
│   │   │   └── README.md       # 本模塊文檔
│   │   ├── coordinator.go      # 協調器
│   │   ├── agent.go            # Agent 核心
│   │   ├── swarm.go            # Swarm 實現
│   │   └── ...
│   ├── kernel/                  # 核心系統
│   └── ...
├── main.go
└── README.md
```

---

## 已完成模塊

### 1. DependencyManager (依賴管理系統) ✅

**功能**：
- 任務依賴圖構建
- 循環依賴檢測
- 拓撲排序執行順序
- 深度限制控制
- 優先級調度
- 事件監聽系統

**狀態**：30/30 測試通過

**文檔**：`internal/agent/dependency/README.md`

---

## 進行中模塊

### 1. Coordinator (協調器)
**位置**：`internal/agent/coordinator.go`

### 2. Swarm System (Swarm 系統)
**位置**：`internal/agent/swarm.go`

### 3. Kernel Components (核心組件)
**位置**：`internal/kernel/`

---

## 設計原則

### 1. Claude Code 借鑒架構
```
crushcl ≠ Claude Code 克隆
crushcl = 基於 Claude Code 架構啟發的自研專案
```

### 2. 介面標準化
```go
type Compressor interface {
    Compact(ctx context.Context, messages []Message) ([]Message, error)
}
```

### 3. Claude Code 4 層壓縮系統
| 層級 | 名稱 | 觸發條件 |
|------|------|---------|
| L1 | 微壓縮 | 每輪結束 + >20 tool results |
| L2 | 自動壓縮 | Token ≥ 85% budget |
| L3 | 完整壓縮 | Token ≥ 95% budget |
| L4 | 會話記憶 | Token ≥ 85% + existing collapses |

---

## 協作 Agent 角色

| Agent | 職責 |
|-------|------|
| Architect | 架構設計、協調、介面標準化 |
| CrushCL | 根據設計進行具體實現 |
| Claude Code | 提供架構啟發（不是直接複製） |
| 其它 AI Agent | 逆向工程、技術結晶提取 |

---

## 上下文管理策略

| 等級 | 閾值 | 行動 |
|------|------|------|
| 正常 | < 50k tokens | 無需行動 |
| 注意 | 50k - 80k | 開始增量摘要 |
| **安全寫入點** | ~85k | **立即寫入 worklog** |
| 警告 | 80k - 120k | 確保日誌已寫入 |
| 危險 | > 120k | 暫停新任務 |

---

## 開發指南

### 編譯
```bash
cd "G:/AI分析/crushcl"
"G:/AI分析/go/bin/go.exe" build ./...
```

### 測試
```bash
cd "G:/AI分析/crushcl"
"G:/AI分析/go/bin/go.exe" test ./internal/agent/dependency/... -v
```

### 運行
```bash
cd "G:/AI分析/crushcl"
go run .
```

---

## 資源

### AI Agent 資源索引
`G:/AI分析/AI_Agent資源/README.md`

| 文檔 | 內容 |
|------|------|
| 01_OpenClaw.md | 250k+ stars |
| 02_Ollama.md | 160k+ stars |
| 03_n8n.md | 181k+ stars |
| 04_Gemini_CLI.md | 96k+ stars |
| 05_Open_WebUI.md | 124k+ stars |
| 06_OWL_Agent框架.md | GAIA冠軍 |
| ... | ... |

---

## 常用路徑

```bash
# CrushCL 項目
cd "G:/AI分析/crushcl"

# 編譯
"G:/AI分析/go/bin/go.exe" build ./...

# AI Agent 資源
G:/AI分析/AI_Agent資源/README.md
```

---

*最後更新：2026-04-05*
*維護者：Architect Agent*
