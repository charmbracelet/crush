# Claude Code Bridge

讓 Crush (CL) 能直接與 Claude Code 通信的橋樑工具。

## 功能

- ✅ 直接執行 Claude Code CLI 命令
- ✅ 支持 CLI 模式 (`-p` 打印模式)
- ✅ 支持流式輸出 (`--output-format stream-json`)
- ✅ 支持 MCP 協議整合
- ✅ 提供 Go API 方便集成

## 安裝 Claude Code CLI

```bash
# macOS
brew install claude-code

# Linux
curl -fsSL https://downloads.anthropic.com/linux_install.sh | sh

# 或從 https://claude.ai/code 下載
```

## 快速開始

### 1. CLI 使用

```bash
# 基本查詢
go run . -p "Explain recursion"

# 使用特定模型
go run . -p "Fix the bug" --model opus

# 限制工具
go run . -p "Read package.json" --tools "Read,Glob"

# JSON 輸出
go run . -p "Summarize project" --output json

# 流式輸出
go run . -p "Write a story" --output stream
```

### 2. Go API 使用

```go
package main

import (
    "context"
    "fmt"
    "github.com/charmbracelet/crushcl/cmd/claudecode-bridge"
)

func main() {
    cli := &bridge.DirectCLI{}
    
    ctx := context.Background()
    response, err := cli.QueryWithOptions(
        ctx,
        "What files are in this directory?",
        "sonnet",
        []string{"Bash", "Read", "Glob"},
    )
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Println(response)
}
```

### 3. MCP 集成

Claude Code 可以作為 MCP Server 運行：

```bash
# 終端 1：啟動 Claude Code MCP Server
claude mcp serve

# 終端 2：通過 MCP 調用
claude mcp add my-server -- npx -y @modelcontextprotocol/server-filesystem .
```

## 架構

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Crush     │────▶│ ClaudeCodeBridge │────▶│  Claude Code    │
│   (CL)     │     │  (Go Bridge)    │     │  CLI / MCP      │
└─────────────┘     └──────────────────┘     └─────────────────┘
```

## 實現模式

### 模式 1：CLI 模式 (最簡單)

```go
cmd := exec.Command("claude", "-p", prompt, "--bare")
output, _ := cmd.Output()
```

### 模式 2：MCP 模式 (最完整)

```go
// Claude Code 作為 MCP Server
cmd := exec.Command("claude", "mcp", "serve")

// 連接 MCP Client
client := mcp.NewClient(...)
session, _ := client.Connect(ctx, transport, nil)

// 調用工具
result, _ := session.CallTool(ctx, params)
```

### 模式 3：Remote Control (遠程控制)

```bash
# Claude Code 支持遠程控制
claude --remote-control "My Session"

# 從其他客戶端連接
claude --connect "My Session"
```

## 測試

```bash
# 測試 CLI 是否可用
claude --version

# 測試 Bridge
go run . -p "Hello, what model are you?" -v
```

## 選項

| 選項 | 說明 | 默認值 |
|------|------|--------|
| `-p` | 提示文本 | 必填 |
| `--model` | 模型 (sonnet/opus/haiku) | sonnet |
| `--tools` | 允許的工具 | 全部 |
| `--output` | 輸出格式 (text/json/stream) | text |
| `-v` | 詳細輸出 | false |

## 與 CL 集成

將 Claude Code Bridge 添加到 CL 的 Agent 協調器：

```go
// internal/agent/coordinator.go

type Coordinator struct {
    claudeCodeBridge *bridge.ClaudeCodeBridge
    // ... other components
}

func (c *Coordinator) QueryClaudeCode(ctx context.Context, prompt string) (string, error) {
    return c.claudeCodeBridge.Query(ctx, prompt,
        bridge.WithModel("sonnet"),
        bridge.WithTools("Read", "Edit", "Bash"),
    )
}
```

## 注意事項

1. **認證**：確保 Claude Code CLI 已登錄 (`claude auth`)
2. **API Key**：需要 ANTHROPIC_API_KEY 環境變量
3. **工具權限**：MCP 模式下工具權限由 Claude Code 管理
4. **超時**：流式輸出模式下需要適當的超時設置

## 參考

- [Claude Code CLI 文檔](https://code.claude.com/docs/en/cli-reference)
- [MCP 協議規範](https://modelcontextprotocol.io/specification)
- [Agent SDK](https://platform.claude.com/docs/en/agent-sdk/overview)
