# Crush + Gemini 3 Pro `Corrupted thought signature` 问题分析报告

- 版本：crush v0.77.0（本地源码树 `../crush`）
- 依赖：`charm.land/fantasy v0.31.1`，底层 `google.golang.org/genai v1.60.0`
- 现象：搭配 Gemini 3 Pro，运行一段时间后频繁报 `bad request: Corrupted thought signature`，且同一会话持续复现、跨重启仍失败（日志 `1.txt`/`2.txt`，同一 `session_id e1d37cb9` 自 10:16 起反复失败）。

---

## 1. 结论（TL;DR）

`Corrupted thought signature` 是 Gemini 的 API 报错。Gemini 3 / 2.5 思考模型在返回 function call 时，会在对应 part 上附带加密的 `thoughtSignature`，**后续请求必须把每个签名原样、按 part 一一对应地回传**。

crush 的消息模型每条 assistant 消息**只保存一个 `ReasoningContent`**，无法表达「多个签名各属于不同 part」。当 Gemini 3 Pro 发起**多个/并行 function call** 时：

1. crush 把多个签名**拼接成一个字符串**回传 → 签名损坏；
2. 损坏的签名被**持久化进 SQLite 会话历史**，之后每次请求都重放被污染的历史 → 错误持续复现、跨重启仍失败。

---

## 2. fantasy 侧的契约（关键约束）

文件：`charm.land/fantasy@v0.31.1/providers/google/google.go`

### 2.1 解析响应（响应 → fantasy 事件）
- `Stream`（行 728–826）与 `mapResponse`（行 1362–1404）：**每个 function call 的签名都作为一次独立的 `OnReasoningEnd` 事件**抛出，并携带 `ReasoningMetadata{Signature, ToolID: <该 call 的 id>}`。
- 即：**一次助手回合会触发多次 `OnReasoningEnd`，每次对应一个工具调用的签名**；纯文本回合的签名则 `ToolID == ""`。

### 2.2 回放请求（fantasy 消息 → genai 请求）
- `toGooglePrompt`（行 414–477）按 `Content` 顺序遍历 assistant parts：
  - 遇到带 google 元数据的 `ReasoningPart`，把签名暂存到 `currentReasoningMetadata`（本身不产出 genai part）；
  - 在**紧随其后的下一个 text / toolCall part** 上设置 `Part.ThoughtSignature`，然后清空暂存。
- **结论：签名与 part 的对应完全依赖顺序**——每个签名必须放在它自己的 `ReasoningPart` 里，且紧贴它所属的那个 part。没有 google 元数据的 reasoning part 会被直接跳过（行 425–428），不会误挂。

---

## 3. crush 侧的缺陷（真正的 bug）

`internal/message/content.go` 的 getter `ReasoningContent()`（行 153）只返回**第一个** reasoning part，且所有 `Append*` 都作用于这个唯一的 reasoning part —— 模型层面只有一个 `ReasoningContent`。

### 缺陷 1：多个签名被拼接成一个（核心触发点）
- `internal/agent/agent.go:884-888` 每次 `OnReasoningEnd` 调用 `AppendThoughtSignature`。
- `content.go:270-285` 的 `AppendThoughtSignature` 执行 `c.ThoughtSignature + signature`，把 N 个不同 base64 签名首尾相连成一个串，`ToolID` 只保留最后一个。
- 回放时 `ToAIMessage`（`content.go:520-525`）把这坨拼接串作为**单个**签名发回 → `Corrupted thought signature`。

### 缺陷 2：工具调用 part 不携带各自签名
- `ToolCall` 结构体（`content.go:101-107`）无签名字段。
- `ToAIMessage`（`content.go:528-535`）重建 tool call 时不带任何 google 元数据。每个 function call 的独立签名无处安放。

### 缺陷 3：签名被 `if reasoning.Thinking != ""` 门控丢弃
- `ToAIMessage:510` 仅在思考文本非空时才发出 reasoning part（连同签名）。
- Gemini 工具回合常常签名非空但思考文本为空，此时签名整体不回传。

### 缺陷 4：mutator 静默清空签名
逐字段重建结构体时未拷贝签名字段，会在回合中途擦除已存签名：
- `FinishThinking`（`content.go:316`）：未拷贝 `ThoughtSignature`/`ToolID`/`ResponsesData`。
- `AppendReasoningContent`（`content.go:249`）：未拷贝 `ThoughtSignature`/`ToolID`/`Signature`/`ResponsesData`。
- `SetReasoningResponsesData`（`content.go:302`）：未拷贝 `ThoughtSignature`/`ToolID`/`Signature`。
- 若给 `ToolCall` 加签名字段，`FinishToolCall`（346）、`AppendToolCallInput`（362）同样会擦除，需一并修。

### 缺陷 5：顺序错位
`ToAIMessage` 把 part 重排为 `text → reasoning → 所有 toolcall`，与 fantasy 要求的「签名紧贴其 part」不符。

---

## 4. 为什么「跑一段时间后必现且持续」

- 早期简单回合（纯文本 / 单工具调用）拼接退化为单签名，多数能蒙混过去；
- 一旦 Gemini 3 Pro 发起**多个/并行 function call**，签名被拼接 → 损坏；
- 关键：**损坏的拼接签名被持久化进 SQLite 会话历史**，之后该会话每次请求都重放被污染的历史 → 错误持续复现，甚至跨重启（与日志中同一 `session_id` 反复失败完全吻合）。

> 排除项：`agent.go:800` 的 `prepared.Messages[i].ProviderOptions = nil` 是**消息级** `Message.ProviderOptions`（cache-control 用），与签名所在的 **part 级** `ReasoningPart.ProviderOptions[google]` 不是同一字段，已核实不影响签名。

---

## 5. 修复方案

核心思路：让 crush 模型能**逐工具调用**保存签名，并在 `ToAIMessage` 中按 fantasy 要求的顺序（每个签名一个独立 `ReasoningPart`，紧贴其 part）回放。

### 改动 1 — `internal/message/content.go`
- `ToolCall` 增加字段 `ThoughtSignature string` `json:"thought_signature,omitempty"`。
- `FinishToolCall`(346)、`AppendToolCallInput`(362)、`AddToolCall` 重建时保留 `ThoughtSignature`。
- 修复 `FinishThinking`(316)、`AppendReasoningContent`(249)、`SetReasoningResponsesData`(302)：重建时拷贝 `ThoughtSignature`/`ToolID`/`ResponsesData`/`Signature`，不再清空。
- 新增 `SetToolCallThoughtSignature(id, sig string)`。
- **重写 `ToAIMessage` 的 Assistant 分支**：
  - 思考/文本签名（`ToolID==""`）：发一个 `ReasoningPart`，**仅当 `ThoughtSignature != ""` 时**才写 `ProviderOptions[google]`，随后发 text part；
  - 每个 tool call：若其 `ThoughtSignature != ""`，**先发只含该签名的 `ReasoningPart`**（`ReasoningMetadata{Signature, ToolID: call.ID}`），紧接着发该 `ToolCallPart`。

### 改动 2 — `internal/agent/agent.go`（Stream 闭包内）
- 新增 `pendingThoughtSigs := map[string]string{}`。
- `OnReasoningEnd`(877) 处理 google 元数据：`ToolID != ""` 时存 `pendingThoughtSigs[ToolID] = Signature`（不再拼接）；`ToolID == ""` 时才 `AppendThoughtSignature(sig, "")`。
- `OnToolCall`(923)（终态）创建 `message.ToolCall` 时设置 `ThoughtSignature = pendingThoughtSigs[tc.ToolCallID]`。

### 改动 3 — 测试
`internal/message` 增加 `ToAIMessage` 单测：构造「思考 + 2 个并行工具调用、各带不同签名」的 assistant 消息，断言输出顺序为 `reasoning(sig_text)?, text, reasoning(sig1)+toolcall1, reasoning(sig2)+toolcall2`，每个 `ReasoningPart` 仅含单个签名且 `ToolID` 正确。

### 已损坏会话说明
此修复只防止**新回合**污染；已写入旧会话历史的「拼接签名」无法还原，受影响会话需**新开 session**。

### 验证
`go build ./...` + `go test ./internal/message/... ./internal/agent/...`；再用 Gemini 3 Pro 跑含多次并行工具调用的长会话回归确认不再报错。
