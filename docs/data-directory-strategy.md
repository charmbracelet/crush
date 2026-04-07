# Data Directory Strategy

## 背景

Crush 采用集中存储策略管理项目数据。项目数据（会话、记忆、日志等）存储在用户目录下的集中位置，而不是项目目录中。

## 存储结构

```
~/.local/share/crush/                    # Unix/Linux/macOS
%LOCALAPPDATA%\crush\                    # Windows

├── crush.json                           # 全局数据配置
├── projects.json                        # 项目索引
└── projects/
    ├── crush-a1b2c3/                    # 项目 A (基于 git root 的 slug)
    │   ├── crush.db                     # 会话数据库
    │   ├── memory/                      # 长期记忆
    │   │   ├── MEMORY.md                # 记忆索引
    │   │   └── *.md                     # 记忆文件
    │   └── logs/
    │       └── crush.log
    └── myproject-d4e5f6/                # 项目 B
        └── ...
```

## 项目配置目录

项目配置仍保留在项目目录中，可以提交到版本控制：

```
<workspace>/.crush/
├── crush.json                           # 项目级配置 (可提交)
└── plugins/                             # 项目级插件
```

## 设计原则

1. **集中存储项目数据**: 会话、记忆、日志存储在集中目录，避免敏感数据被误提交
2. **按 git root 隔离**: 同一仓库的所有 worktree 共享同一数据目录
3. **项目配置本地化**: 项目级配置文件可提交，便于团队共享
4. **向后兼容**: 支持通过环境变量或配置覆盖默认路径

## 路径解析

### 项目数据目录

```go
ProjectDataDir(workingDir) -> ~/.local/share/crush/projects/<slug>/
```

slug 生成规则：
1. 优先使用 git root（支持 worktree 统一）
2. 格式: `<basename>-<hash6>`
3. `basename`: 项目目录名（非法字符替换为下划线）
4. `hash6`: 路径 sha256 哈希的前 6 个字符

### 项目配置路径

```
<workspace>/.crush/crush.json
```

配置文件直接位于工作目录下，可通过 `lookupConfigs` 向上查找。

## 环境变量覆盖

| 环境变量 | 说明 |
|---------|------|
| `CRUSH_GLOBAL_DATA` | 全局数据目录 |
| `CRUSH_WORKSPACE_CWD` | 显式指定工作空间路径 |
| `PWD`, `PROJECT_ROOT` 等 | 工作空间路径的备选来源 |

## 与 Claude Code 的对比

| 特性 | Claude Code | Crush |
|------|-------------|-------|
| 记忆存储 | `~/.claude/projects/<hash>/memory/` | `~/.local/share/crush/projects/<slug>/memory/` |
| 会话存储 | 同上 | 同上 |
| git root 支持 | 是 | 是 |
| worktree 共享 | 是 | 是 |
| 项目配置 | 无 | `<workspace>/.crush/crush.json` |

## 迁移指南

旧版本数据存储在 `<workspace>/.crush/` 中。升级后：

1. 新数据自动存储到集中目录
2. 旧数据不会自动迁移
3. 可手动复制 `.crush/memory/` 到新位置
4. 或运行 `crush` 时自动创建新目录
