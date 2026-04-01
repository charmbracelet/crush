# Claude Code 借鉴能力对齐收敛计划（当前状态）

## 已完成（已在代码落地）

1. **结构化子任务结果协议**
   - `message.ToolResult` 支持 `subtask_result` / `reducer` metadata helper：`internal/message/auto_mode.go`
   - `runSubAgent` 成功/失败统一回填 `child_session_id/parent_tool_call_id/status`：`internal/agent/coordinator.go`
   - 合成 tool_result（取消/权限/错误）补齐 subtask 状态：`internal/agent/agent.go`
   - ACP 与 `session show --json` 已投影 `subtaskResult` 与 `reducer`：`internal/acp/handler.go`, `internal/cmd/session.go`

2. **统一编排状态机（mode）**
   - 统一 `ModeState/ModeTransition`：`internal/session/mode_state.go`
   - session 更新路径已用 transition：`internal/session/session.go`

3. **时间线基础设施 + ACP/UI 转发**
   - timeline service/event model：`internal/timeline/service.go`, `internal/timeline/helpers.go`
   - app 自动汇聚 session/toolruntime -> timeline：`internal/app/timeline.go`
   - ACP 实时转发 timeline_event，UI 可展示 timeline：`internal/acp/handler.go`, `internal/ui/model/timeline_view.go`

4. **TaskGraph / DAG 调度器（P0）**
   - 新增 DAG 校验与分层拓扑：`internal/agent/taskgraph/taskgraph.go`
   - `agent` 工具支持 `tasks[]`（`id/prompt/subagent_type/depends_on/description`）并走 `runTaskGraph`：`internal/agent/agent_tool.go`, `internal/agent/coordinator.go`
   - 实现依赖失败传播与分层并行执行：`internal/agent/coordinator.go`

5. **结构化 fan-in reducer（P0）**
   - 新增 reducer 结构与聚合实现：`internal/agent/reducer/service.go`
   - TaskGraph 完成后聚合 `summary/artifacts/risks/next_actions/confidence` 并写入 metadata：`internal/agent/coordinator.go`

6. **权限同步协议（P1）**
   - 新增 `AuthoritySessionID` 字段（permission + ACP）：`internal/permission/permission.go`, `internal/acp/types.go`
   - 子会话权限请求自动上送父会话裁决（无父会话时回落当前会话）：`internal/agent/tools/permission_helper.go`
   - ACP `session/request_permission` 转发 `authoritySessionId`：`internal/acp/client.go`

7. **工具池风险分级预过滤（P1）**
   - 新增工具风险级别（read/write/execute/network/delegation）：`internal/agent/plan_mode.go`
   - Plan 模式按风险层过滤，仅保留只读工具 + `request_user_input` + `plan_exit`，明确剔除 delegation/network/write/execute：`internal/agent/plan_mode.go`, `internal/agent/coordinator.go`

8. **历史/全局搜索（P2）**
   - 新增消息检索 SQL：`SearchMessages`：`internal/db/sql/messages.sql`
   - 新增 history 搜索服务：`internal/history/search.go`
   - 新增 `history_search` 工具：`internal/agent/tools/history_search.go`
   - 新增 CLI：`crush session search <query>`：`internal/cmd/session.go`

9. **长期记忆层（P2）**
   - 新增可审计、默认保守的长期记忆服务（`entries.json` + `audit.log`）：`internal/memory/service.go`
   - 新增 `long_term_memory` 工具（store/get/delete/search/list）：`internal/agent/tools/memory.go`
   - 工具接入与风险策略联动：`internal/agent/coordinator.go`, `internal/agent/plan_mode.go`, `internal/config/config.go`

---

## 当前剩余缺口

1. **任务级治理增强（未做）**
   - 仍缺并发上限、重试预算、超时预算、熔断/舱壁等治理策略。

2. **角色化 orchestrator（未做）**
   - 已有子代理与 DAG，但未形成 planner/reviewer/executor 的显式角色协作协议。

3. **上下文压缩流水线增强（部分）**
   - 已有最小可用压缩触发，但未形成 Claude 风格的 micro/autocompact/reactive/collapse 全流程。

---

## 本轮验证

- `go -C D:/code/copilot-refs/crush test ./...` ✅

> 说明：当前工作区已完成 P0/P1/P2（除治理增强与角色化 orchestrator 等后续项）。
