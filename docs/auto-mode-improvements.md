# Auto Mode 改进评估（2026-03）

本文档记录对 Auto Mode 改进提案的复核结论，并给出当前建议实施项。

## 结论摘要

- 最高优先级是补齐关键安全函数单元测试，覆盖 `internal/autopermission/service.go` 与 `internal/agent/auto_classifier.go` 的纯函数。
- `parseQuickClassifierDecision` 当前仅接受精确 `ALLOW`，存在真实鲁棒性缺陷，应立即修复并配套测试。
- 在关键决策路径补充 `slog.Debug` 是低成本、可落地的可观测性改进。

## 建议实施项

### P0：安全关键纯函数测试补齐

目标是把“高影响、零覆盖”的决策函数纳入稳定回归测试，优先覆盖：

- `isSafeReadOnlyBashRequest`
- `isSafeWorkspaceWrite`
- `isSensitiveWorkspacePath`
- `isHighRiskBashRequest`
- `isAlwaysManual`
- `parseQuickClassifierDecision`
- `parseAutoClassification`
- `extractFirstJSONObject`
- `parseAutoClassificationTextFallback`
- `suspiciousToolOutputSnippet`
- `isTrustedLocalReadOnlyToolResult`

测试风格保持与现有代码一致：表驱动、`t.Parallel()`、`require` 断言。

### P0：修复 quick classifier 解析鲁棒性

现状问题：`parseQuickClassifierDecision` 只接受精确文本 `ALLOW`，面对常见模型输出变体（大小写、包装、轻微格式差异）时会误判为拒绝。

修复原则：

- 保持 fail-closed（无法确定时默认拒绝）。
- 接受常见等价输出（如规范化后 `ALLOW`、`decision: allow`、`<block>no</block>`、`{"allow_auto": true}`）。
- 明确拒绝含歧义或否定语义的自然语言句子。

### P2：关键路径调试日志（`slog.Debug`）

在自动批准的关键分支补充调试日志即可，不引入新的事件系统：

- 快速路径命中（allowlist、accept-edits、只读 bash）。
- 分类器允许/阻止。
- 分类器异常与降级路径。
- 熔断状态变更。

## 不建议实施项

以下提案在当前 Crush 产品形态下收益低或存在明显副作用，建议不做：

- 审计事件系统：Crush 是本地 CLI 工具，不是 SaaS 审计平台；已有 PostHog 事件能力。关键路径 `slog.Debug` 已足够支撑排障。
- Headless 模式熔断：`RunNonInteractive` 通过 `AutoApproveSession()` 直接绕过权限系统与分类器，所谓 headless 熔断场景在当前实现中不存在。
- 分类器决策缓存：分类器输入包含最近 16 条上下文消息，按 `tool:action:path` 缓存会牺牲上下文安全性，存在误放行风险。
- UI 统计面板：与 Auto Mode “尽量不打断用户”的目标冲突，且增加交互噪音。
- 撤销/回滚机制：文件回滚已有 Git 最佳实践；通用 bash 回滚不可判定，工程上不可可靠实现。
- 结构化规则引擎：基于代码的 glob/regex 规则会与现有自然语言规则和 `isAlwaysManual`/allowlist 逻辑重复，复杂度高于收益。
- 配置大幅可调：当前阈值（3/20）与提醒周期（5）采用成熟默认值，暂无明确用户需求驱动扩展。

## 当前执行顺序

1. 先重构并加固 `parseQuickClassifierDecision`，确保 fail-closed 前提下兼容常见输出变体。
2. 补齐 `autopermission`、`auto_classifier`、`auto_guard` 关键纯函数测试并跑通相关测试集。
3. 最后补充少量关键路径 `slog.Debug`（单独小变更，避免与逻辑改动耦合）。

## 参考

- `internal/autopermission/service.go`
- `internal/agent/auto_classifier.go`
- `internal/agent/auto_guard.go`
- `internal/agent/auto_mode_reminder_test.go`
- `internal/message/auto_mode_test.go`
