# Claude Code 能力与思路综合分析（面向 Crush 吸收）

> 日期：2026-03-31  
> 范围：基于 `claude-code`、`vscode-copilot-chat`（Claude 集成）与 `crush` 现状代码调研结果，提炼可迁移能力与实施路线。

## 1. 执行摘要

- Claude 生态里“subagent/agent-swarm”本质是**分层编排**：外层会话串行，内层任务并发，依赖结构化 ID 与消息通道完成 fan-out/fan-in。
- `agent-swarm` 在仓库中**没有字面模块名**，但存在等价实现：`team-lead + teammate + mailbox + tasks`。
- Crush 当前已具备：Plan/Auto 等模式、权限门控、子代理委派、MCP、会话持久化、重试/取消。
- Crush 当前短板主要在：缺统一任务图（DAG）、缺结构化 fan-in 聚合器、缺角色化调度器、缺任务级可靠性治理（熔断/舱壁）。
- 建议优先吸收：**结构化子任务结果协议 + 轻量 TaskGraph + 统一编排层状态机**，其次是权限同步、上下文压缩流水线、可观测性时间线。

---

## 2. Claude Code 的 subagent 与“swarm”编排逻辑（复盘）

## 2.1 Subagent（VS Code Claude 集成）

### 编排链路

`Chat Session Provider -> AgentManager -> ClaudeCodeSession(queue) -> Claude SDK -> tool_use/tool_result -> history builder`

关键证据：
- 入口与装配：`vscode-copilot-chat/src/extension/chatSessions/vscode-node/chatSessions.ts:181-203`
- 请求入口：`.../claudeChatSessionContentProvider.ts:237-305`
- 会话路由：`.../claudeCodeAgent.ts:63-109`
- 单会话串行队列 `_promptQueue`：`.../claudeCodeAgent.ts:158-173,311-373,538-589`
- 工具调用/回填：`.../claudeCodeAgent.ts:842-963`
- 子代理结果回挂：`.../claudeCodeSessionService.ts:304-349`, `.../chatHistoryBuilder.ts:420-541`

### 核心机制

1. **外层串行、内层并发**：会话级严格顺序，工具/子代理可并发。  
2. **双层隔离**：主 session 状态 + subagent transcript（`subagents/*.jsonl`）。  
3. **结构化关联**：`tool_use_id`、`parent_tool_use_id`、`agentId` 贯通生命周期。  
4. **两阶段汇总**：运行时 UI 闭环 + 历史回放重建。  
5. **权限边界统一入口**：`canUseTool -> ClaudeToolPermissionService`。

## 2.2 “agent-swarm”等价实现（Claude Code Team 模型）

### 结论

仓库内无字面 `agent-swarm` 模块名，但存在等价群体编排：**Team Lead（编排者）+ Teammate（执行者）+ Mailbox（通信）+ Tasks（调度状态）**。

关键证据：
- swarm 开关：`claude-code/utils/agentSwarmsEnabled.ts:24-44`
- team 创建：`claude-code/tools/TeamCreateTool/TeamCreateTool.ts:128-236`
- worker 触发：`claude-code/tools/AgentTool/AgentTool.tsx:282-316`
- 后端选择（in-process/pane）：`claude-code/tools/shared/spawnMultiAgent.ts:1040-1093`
- worker 主循环：`claude-code/utils/swarm/inProcessRunner.ts:1047-1417`
- 执行器：`claude-code/tools/AgentTool/runAgent.ts:748-757`
- fan-in 收敛：`claude-code/hooks/useInboxPoller.ts:802-865`, `claude-code/utils/teammateMailbox.ts:134-191`
- 权限同步：`claude-code/utils/swarm/permissionSync.ts:676-783`

### 编排思路

- **fan-out**：leader 根据 task 拆分 spawn 多 worker。
- **state**：task claim/lock + mailbox 文件消息。
- **fan-in**：leader 轮询 inbox 聚合 worker 结果。
- **终止/取消**：AbortController + shutdown 协议 + TeamDelete 活跃保护。
- **重试策略**：偏基础（锁重试/轮询），非强任务自动重跑框架。

---

## 3. Claude Code 功能与特色全景（可借鉴视角）

## 3.1 产品能力面

1. **多入口运行时**：交互 TUI + 非交互/SDK 通道并存（`claude-code/main.tsx:797-815,1235-1252,2217-2242`）。
2. **场景化启动 fast-path**（daemon/bg/tmux/remote）提升冷启动与专用场景效率（`claude-code/entrypoints/cli.tsx:108-209,247-274`）。
3. **键位上下文系统**：可按上下文切换行为，支持用户覆盖（`claude-code/keybindings/schema.ts:12-32,64-172`）。
4. **历史检索双通道**：内联 + 模态 fuzzy picker（`claude-code/hooks/useHistorySearch.ts:237-257`, `.../HistorySearchDialog.tsx:65-109`）。
5. **多模态输入细节完善**：粘贴/图片/附件处理完备（`claude-code/hooks/usePasteHandler.ts:118-176,241-268`）。

## 3.2 工具与安全

1. **统一 Tool 契约**：并发安全、是否破坏性、权限检查、渲染接口统一（`claude-code/Tool.ts:362-635,757-792`）。
2. **工具池治理**：内置 + MCP 合并，deny 规则预过滤，simple 模式裁剪（`claude-code/tools.ts:193-367`）。
3. **权限模式分层**：default/plan/bypass/auto + 危险规则剥离（`claude-code/main.tsx:1390-1411,1747-1771`; `.../permissionSetup.ts:1371-1405,1462-1493`）。
4. **Trust-first**：先信任再放开环境变量/MCP/外部包含（`claude-code/interactiveHelpers.tsx:125-185`）。
5. **运行时硬化**：如 Windows PATH 劫持防护（`claude-code/main.tsx:588-592`）。

## 3.3 会话、上下文、记忆、可观测性

1. **会话持久化分层**：JSONL + sidechain + 远端同步回填（`claude-code/utils/sessionStorage.ts:1128-1265,1587-1622,1632-1711`）。
2. **上下文压缩流水线**：micro/autocompact/reactive/collapse（`claude-code/query.ts:396-536,1065-1256`）。
3. **SessionMemory 双轨记忆**（有 gate）（`claude-code/services/SessionMemory/sessionMemory.ts:277-325,357-375`）。
4. **成本/性能可观测**：会话成本追踪 + 渲染指标（`claude-code/cost-tracker.ts:87-123,143-175,228-243`; `.../interactiveHelpers.tsx:315-363`）。

## 3.4 扩展生态

- MCP 从配置、策略过滤、连接管理到协议桥接较完整（`claude-code/main.tsx:1413-1522,2687-2730`; `claude-code/entrypoints/mcp.ts:59-170`）。

---

## 4. Crush 现状基线（对照吸收）

| 维度 | 现状判定 | 关键证据 |
|---|---|---|
| 执行模式（default/plan/auto/yolo） | 部分具备 | `crush/internal/session/session.go:26-31`; `crush/internal/agent/plan_mode.go:12-37,106-137`; `crush/internal/acp/handler.go:716-717` |
| 工具权限 | 已具备 | `crush/internal/permission/permission.go:287-329,384-390`; `crush/internal/autopermission/service.go:166-167,271-305` |
| 会话持久化 | 已具备 | `crush/internal/db/sql/sessions.sql:1-40,82-92`; `crush/internal/session/session.go:330-390` |
| 上下文/记忆 | 部分具备 | `crush/internal/agent/prompt/prompt.go:167-185`; `crush/internal/agent/agent.go:1237-1412` |
| 任务并发 | 部分具备 | `crush/internal/agent/coordinator.go:180-183`; `crush/internal/agent/agent.go:2184-2210` |
| 子代理 | 已具备 | `crush/internal/agent/agent_tool.go:29-37,73-80`; `crush/internal/agent/coordinator.go:1505-1563`; `crush/internal/config/config.go:666-696` |
| MCP | 已具备 | `crush/internal/config/config.go:217-263`; `crush/internal/agent/tools/mcp/init.go:168-207,529-579` |
| 可观测性 | 部分具备 | `crush/internal/event/event.go:50-60,79-119`; `crush/internal/agent/event.go:10-37` |
| UI 交互 | 已具备 | `crush/internal/ui/model/ui.go:1614-1669`; `crush/internal/ui/dialog/commands.go:430-462` |

### 关键差距（与 Claude/Swarm 对照）

1. 缺统一 **TaskGraph/DAG** 与依赖拓扑。  
2. 缺结构化 **fan-in reducer**（当前偏文本回收）。  
3. 缺角色化 orchestrator（planner/reviewer/executor 可编排）。  
4. 缺任务级治理（熔断/舱壁/预算）与跨子任务统一策略。  
5. 子代理结果协议化程度不足（可机器验证的 schema 不够强）。

---

## 5. 建议吸收清单（按优先级）

## P0（建议优先落地）

### 1) 结构化子任务结果协议（Result Contract）
- **目标**：子代理输出统一为 `task_id/status/artifacts/risks/confidence/next_action`。
- **收益**：提升 fan-in 稳定性、减少纯文本汇总误差。
- **Crush 落点**：`agent_tool.go` + `coordinator.runSubAgent` + tool result formatter。
- **参考**：Claude 的 `tool_use_id/agentId` 回挂与历史重建（`claudeCodeSessionService.ts:304-349`）。

### 2) 轻量 TaskGraph（非重型工作流引擎）
- **目标**：支持 task 依赖、并发度、终止条件、失败传播。
- **收益**：从“会话树”升级为“可控并发图”。
- **Crush 落点**：`coordinator.go`（调度层），配合 `toolruntime` 状态机。
- **参考**：swarm 的 tasks+mailbox 协同（`claude-code/utils/tasks.ts:541-692`）。

### 3) 编排层统一状态机（Mode + Permission + Delegation）
- **目标**：将 plan/auto/yolo/permission_mode 融合为统一、可观测状态机。
- **收益**：减少边界分叉和策略冲突。
- **Crush 落点**：session model + ACP handler + UI commands。

### 4) 事件时间线（Timeline Tracing）
- **目标**：统一记录 prompt/tool/permission/subagent span。
- **收益**：提升定位复杂编排问题能力。
- **Crush 落点**：`internal/event`、`internal/agent/event.go`、ACP 事件桥。

## P1（中期增强）

### 5) 权限同步协议（主代理裁决子代理敏感请求）
- 参考 `permissionSync.ts:676-783`，避免子代理绕开主策略。

### 6) 上下文压缩流水线
- 在现有 summarization 基础上增加 reactive/microcompact 策略，降低长会话退化。

### 7) 工具池治理增强
- 增加预过滤层（按模式、风险级、项目策略）在工具曝光前裁剪。

### 8) 结构化 I/O 通道
- 为 ACP/CLI 增加统一 request/response/cancel 信封格式，减少跨通道分叉行为。

## P2（后续可选）

### 9) 长期记忆层（可关闭）
- 会话摘要索引 + 语义召回，默认保守开启并可审计。

### 10) 高级 UX 检索
- 历史 fuzzy picker、全局快速检索、并发任务可视化。

---

## 6. 建议路线图

## 阶段 1（4-6 周）：稳态基础

- P0-1 结构化子任务结果协议
- P0-3 编排状态机统一
- P0-4 时间线观测最小闭环

**验收指标**：
- 子代理结果解析失败率下降
- 跨模式行为一致性（回归用例）提升
- 复杂任务排障时间下降

## 阶段 2（4-8 周）：并发能力增强

- P0-2 轻量 TaskGraph
- P1-5 权限同步
- P1-7 工具池预过滤

**验收指标**：
- 并发子任务成功率
- 错误隔离能力（局部失败不拖垮全局）
- 人工确认次数与误放行率平衡

## 阶段 3（持续演进）

- P1-6 上下文压缩流水线
- P2-9 长期记忆（可选）
- P2-10 高级 UX 检索

**验收指标**：
- 长会话 token 成本下降
- 上下文超限中断率下降
- 用户任务完成时长下降

---

## 7. 不建议直接照搬的点

1. 不建议一次性引入重型 swarm runtime（成本与复杂度高）。
2. 不建议默认开启激进自动记忆（易引入错误召回与隐私风险）。
3. 不建议先做 UI 花哨功能，先保证编排内核与治理能力。

---

## 8. 结论

Crush 已有扎实基础（权限、模式、子代理、MCP、持久化），下一步关键不是“再加工具”，而是把多代理执行从“可用”提升到“可编排、可治理、可观测”。Claude Code 最值得吸收的是其**结构化编排思路**（ID 关联、消息通道、结果聚合、状态治理），而非表层功能堆叠。