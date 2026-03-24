# Data Directory Strategy

## 背景

ACP 客户端在 Windows 上可能从 `C:\Windows\System32` 启动 `crush acp`。当前默认策略会把 `options.data_directory` 解析为 `<cwd>/.crush`，导致尝试创建 `C:\Windows\System32\.crush` 并失败（权限不足），表现为 ACP 端报错：

- `Internal error: "server shut down unexpectedly"`
- `Failed to create data directory ... Access is denied`

本方案统一梳理 Crush 数据归属，明确“项目级数据”与“用户级数据”的边界，并给出在 ACP 场景下的落地实现。

---

## 现状盘点（按数据类型）

### 一、项目级数据（应与 workspace 绑定）

这些数据由 `options.data_directory` 决定，默认在项目 `.crush/` 下：

1. **会话与消息数据库**
   - `internal/db/connect.go`：`<dataDir>/crush.db`
   - 包含 `sessions/messages/files/read_files` 等表（见 `internal/db/migrations/*`）
2. **workspace 覆盖配置**
   - `internal/config/load.go`：`workspacePath = <dataDir>/crush.json`
   - ACP 模型切换写入 `ScopeWorkspace`（`internal/acp/handler.go`）
3. **项目日志**
   - `internal/config/load.go` + `internal/log/log.go`：`<dataDir>/logs/crush.log`
4. **项目命令**
   - `internal/commands/commands.go`：`<dataDir>/commands`
5. **初始化标记**
   - `internal/config/init.go`：`<dataDir>/init`
6. **agentic fetch 临时目录**
   - `internal/agent/agentic_fetch_tool.go`：`os.MkdirTemp(<dataDir>, ...)`
7. **本地插件与工具（仍在项目目录）**
   - `internal/cmd/plugin.go`、`internal/plugin/local_tools.go`：`<workingDir>/.crush/plugins`、`<workingDir>/.crush/tools`

> 结论：这些数据都需要“按 workspace 隔离”。

### 二、用户级数据（天然全局）

1. **全局配置**
   - `config.GlobalConfig()` → `~/.config/crush/crush.json`（或平台等价路径）
2. **全局数据配置**
   - `config.GlobalConfigData()` → `~/.local/share/crush/crush.json`（Windows 为 `%LOCALAPPDATA%\crush\crush.json`）
3. **项目索引注册表**
   - `internal/projects/projects.go`：`<global-data-dir>/projects.json`
4. **全局 skills / 全局 commands**
   - `internal/config/load.go`、`internal/commands/commands.go`

> 结论：这类数据应继续放用户目录，不应与某个具体项目绑定。

---

## 核心设计原则

1. **按数据“归属”分层，而不是按“启动位置”分层**。
2. **项目级数据必须可追溯到 workspace**（同项目复用，同名项目隔离）。
3. **用户级数据只保存跨项目状态**（如全局配置、项目索引）。
4. **ACP headless 启动必须容错**：即使启动 cwd 不可写，也要能落到“安全路径”并继续服务。

---

## 方案设计

### 1) 项目级数据目录解析策略

`options.data_directory` 默认保持：

- 正常场景：`<workspace>/.crush`

新增 fallback（仅当启动 cwd 不安全时触发）：

- 目标路径：`<global-data-root>/workspaces/<workspace-slug-hash>/`
- 其中 `<global-data-root> = dirname(GlobalConfigData())`
- `<workspace-slug-hash>` 由 `workspaceIdentity` 生成（见下节）

触发条件（unsafe cwd）：

- Windows：cwd 位于 `C:\Windows\System32`（含其子目录）
- Unix-like：cwd 为 `/`
- cwd 为空或 `.`（无有效工作目录）

### 2) 如何区分“这个全局目录属于哪个项目”

引入 **workspace identity** 概念：

- 优先读取环境变量（按优先级）：
  - `CRUSH_WORKSPACE_CWD`
  - `ZED_WORKSPACE_ROOT`
  - `ZED_WORKTREE_ROOT`
  - `ZED_CWD`
  - `VSCODE_CWD`
  - `PROJECT_ROOT`
  - `WORKSPACE_ROOT`
  - `INIT_CWD`
  - `PWD`
- 对 identity 做标准化（`Abs + Clean`）
- 若 identity 仍不安全，则回退到传入 workingDir
- 最终目录名：`<basename>-<hash12>`
  - `basename`：workspace 最后一段，非法字符转 `_`
  - `hash12`：`sha256(normalizedPath)` 前 12 个 hex 字符

这样可同时满足：

- 可读性：目录里能看到项目名
- 唯一性：同名不同路径不会冲突

### 3) 项目索引注册（projects.json）

`projects.json` 的 `path` 字段应记录 **workspace identity**，而不是“可能为 System32 的启动 cwd”。

这样 `crush projects` 能反映真实项目，而不会出现无意义的 `C:\Windows\System32`。

---

## 实施计划

### 阶段 A（本次实现）

1. 在配置层实现 unsafe cwd 检测与全局 workspace fallback。
2. 引入 workspace identity 解析与目录名生成策略。
3. 在 app 初始化后，projects 注册改用 `store.WorkingDir()`（已被 identity 归一）。
4. 增加单测覆盖：
   - unsafe cwd 判定
   - workspace slug 稳定性
   - env 优先级与回退行为

### 阶段 B（后续增强）

1. 将 ACP 会话的 cwd 显式持久化到 `sessions` 表（新增列 `workspace_cwd`）
   - `session/new`、`session/load` 写入
   - `session/list` 优先返回持久化 cwd
2. 为编辑器适配提供文档：推荐在 ACP 启动时传入 `--cwd` 或 `CRUSH_WORKSPACE_CWD`。
3. 根据需要增加“数据迁移工具”（把旧 `.crush` 数据迁移到新的全局 workspace 路径）。

---

## 兼容性与影响

- 不影响已有项目中 `.crush` 的行为。
- 仅在“启动 cwd 不安全”时走 fallback。
- fallback 目录仍是“项目级隔离”，只是物理上位于用户目录。
- 不改变全局配置文件位置。

---

## 风险与规避

1. **编辑器未暴露 workspace env**
   - 通过多级 env + fallback 减少失败概率
2. **路径哈希不可读**
   - 使用 `basename-hash` 兼顾可读与唯一
3. **旧 session 的 cwd 展示不准确**
   - 阶段 B 通过 DB 持久化彻底解决

---

## 验收标准

1. 在 Windows 中从 `C:\Windows\System32` 启动 `crush acp` 不再因 data dir 权限失败。
2. ACP 可正常完成 initialize/session/new 流程。
3. `options.data_directory` 在 unsafe cwd 下落入 `<global-data-root>/workspaces/...`。
4. `projects.json` 记录真实 workspace identity。
5. 相关单测通过。
