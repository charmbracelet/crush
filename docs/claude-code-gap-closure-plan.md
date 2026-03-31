# Claude Code 借鉴能力对齐收敛计划（当前状态）

## 已完成（已在代码落地）

1. **结构化子任务结果协议**
   - `message.ToolResult` 已支持 `subtask_result` metadata helper：`internal/message/auto_mode.go:31-44,120-165`
   - `runSubAgent` 成功/失败统一回填 `child_session_id/parent_tool_call_id/status`：`internal/agent/coordinator.go:1509-1645`
   - 合成 tool_result（取消/权限/错误）补齐 subtask 状态：`internal/agent/agent.go:988-999,1130-1139`
   - ACP 已投影 `subtaskResult`：`internal/acp/handler.go:542-570`, `internal/acp/types.go:273-307`

2. **统一编排状态机（mode）**
   - 统一 `ModeState/ModeTransition`：`internal/session/mode_state.go:8-73`
   - session 更新路径已用 transition：`internal/session/session.go:370-407`

3. **时间线基础设施 + ACP/UI 转发**
   - timeline service/event model：`internal/timeline/service.go:11-92`, `internal/timeline/helpers.go:10-91`
   - app 自动汇聚 session/toolruntime -> timeline：`internal/app/timeline.go:21-92`
   - ACP 实时转发 timeline_event：`internal/acp/handler.go:250-355,532-540`
   - UI 已接收并展示 timeline 列表：`internal/ui/model/timeline.go:10-30`, `internal/ui/model/timeline_view.go:15-166`, `internal/ui/model/ui.go:4642-4644`

4. **上下文压缩流水线（最小可用）**
   - 已区分 trigger：normal/recover/proactive：`internal/agent/agent.go:79-104`
   - 预估触发 proactive compaction：`internal/agent/agent.go:341-363`
   - 失败恢复触发 recover summarize：`internal/agent/agent.go:718-747,888-920`
   - 插件 purpose 已支持 proactive_compact：`internal/plugin/plugin.go:136-150`

5. **会话与工具可观测性增强**
   - `session show --json` 已输出 `tool_result.metadata/subtask_result`：`internal/cmd/session.go:602-620,619-699`
   - 子代理导航快捷键文案对齐实际绑定（alt+方向键）：`internal/ui/model/keys.go:285-297`, `internal/ui/dialog/keyboard_shortcuts.go:162-165`

---

## 仍有缺口（尚未实现）

## P0 缺口

1. **TaskGraph / DAG 调度器**（未实现）
   - 当前仍是会话树 + 工具回调，缺任务依赖图、并发上限、失败传播策略。
   - 证据：`internal/agent/coordinator.go:157` 仍有“multiple agents”TODO；全仓无 TaskGraph 实现。

2. **结构化 fan-in reducer**（未实现）
   - 当前是子任务结果 metadata + 文本结果，缺统一 reducer（artifacts/risks/next_actions）。
   - 证据：全仓无 reducer/aggregation schema，ACP 仅透传 `subtaskResult`。

## P1 缺口

3. **权限同步协议（主代理裁决子代理敏感动作）**（部分）
   - 现状：权限统一由 permission service + ACP bridge 处理，但非“leader/worker 同步协议”。
   - 证据：`internal/acp/client.go:21-102` 为通用桥接，不区分 subagent 协作域。

4. **工具池风险分级预过滤层**（部分）
   - 现状：有 plan mode 过滤 + allowed tools 过滤，但无风险分级/预算治理策略。
   - 证据：`internal/agent/coordinator.go:725-868`, `internal/agent/plan_mode.go:106-137`。

## P2 缺口

5. **长期记忆层（跨会话语义召回）**（未实现）
   - 现状：暂无类似 Claude 的长期 memory store 与 recall pipeline。

6. **高级历史检索（全局 fuzzy / 全文检索）**（未实现）
   - 现状：session list/show 可用，但无跨会话语义检索入口。

---

## 下一阶段实施顺序（建议）

1. **TaskGraph 最小骨架（P0）**
   - 新增任务节点结构：`id/type/deps/status/retry_budget/timeout`。
   - 先只接管 `agent` / `agentic_fetch` 子任务编排。

2. **fan-in reducer（P0）**
   - 引入统一结果结构：`summary/artifacts/risks/next_actions/confidence`。
   - ACP 补充 reducer 字段透出，避免纯文本聚合。

3. **权限同步协议（P1）**
   - 子代理高风险请求上送父会话裁决（而非独立直接判定）。

4. **风险分级工具预过滤（P1）**
   - 在 `buildTools` 前注入 risk policy layer（mode + risk + project policy）。

5. **长期记忆与检索（P2）**
   - 先做可关闭、可审计版本，默认保守。

---

## 本轮验证

- `go -C D:/code/copilot-refs/crush test ./internal/agent` ✅
- `go -C D:/code/copilot-refs/crush test ./internal/session ./internal/app ./internal/acp ./internal/agent` ✅
- `go -C D:/code/copilot-refs/crush test ./internal/ui/model` ✅

> 说明：全量 `go test ./...` 在当前工作区曾受其它并行改动影响，已将本轮涉及包全部回归通过。