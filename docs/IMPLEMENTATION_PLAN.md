# OpenCode Plugin 遷移實作計劃

## 1. 概述

本計劃將現有的記憶體和代理相關 Skill 遷移到 OpenCode Plugin 架構，充分發揮 OpenCode Plugin Hooks 系統的能力。

### 1.1 遷移範圍

| 現有 Skill | 目標 Plugin | 優先級 |
|-----------|------------|--------|
| `clawcode-session-memory` | `opencode-memory-plugin` | P0 |
| `enhanced-memory` | `opencode-memory-plugin` | P0 |
| `memory-mcp` | 廢除 | P1 |
| `oh-my-opencode.json` | `opencode-agent-config-plugin` | P0 |

### 1.2 預期收益

| 指標 | Skill 實現 | Plugin 實現 |
|------|-----------|------------|
| Hook 整合深度 | 被動 | 主動 |
| 記憶體捕獲 | 手動觸發 | 自動攔截 |
| 壓縮時機控制 | 外部控制 | 精確 Hook |
| Agent 配置 | 靜態 JSON | 動態 Persona |
| 工具存取控制 | 全域 | Per-Persona |

## 2. Plugin 架構總覽

```
┌─────────────────────────────────────────────────────────────┐
│                    OpenCode Runtime                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐     ┌─────────────────┐            │
│  │ opencode-memory  │     │ opencode-agent   │            │
│  │    -plugin       │     │    -config-plugin │            │
│  └────────┬────────┘     └────────┬────────┘            │
│           │                        │                        │
│  ┌────────▼────────────────────────▼────────┐            │
│  │              Hooks System                   │            │
│  │  • chat.message    • experimental.session.comp│            │
│  │  • tool.execute    • experimental.chat.system │            │
│  │  • agent.config   • experimental.chat.msg    │            │
│  └─────────────────────────────────────────────┘            │
│                            │                                 │
│                            ▼                                 │
│  ┌─────────────────────────────────────────────┐            │
│  │         CrushCL Internal Components          │            │
│  │  • kernel/context (4-tier compression)       │            │
│  │  • agent/coordinator (agent orchestration)   │            │
│  │  • agent/guardian (heartbeat/circuit break) │            │
│  └─────────────────────────────────────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

## 3. 實作階段

### Phase 1: Memory Plugin 核心 (4-6 週)

#### 3.1.1 項目初始化

```bash
# 創建插件項目
mkdir -p opencode-memory-plugin
cd opencode-memory-plugin
npm init -y
npm install @opencode-ai/plugin typescript

# 目錄結構
mkdir -p src/{hooks,memory,storage,types}
mkdir -p test
```

#### 3.1.2 核心類實現

| 檔案 | 職責 | 行數預估 |
|------|------|---------|
| `src/memory/memory-pool.ts` | 記憶池實現 | 300 |
| `src/memory/compact.ts` | 壓縮邏輯 | 200 |
| `src/memory/retrieval.ts` | 檢索邏輯 | 150 |
| `src/storage/sqlite.ts` | SQLite 持久化 | 200 |
| `src/hooks/chat.hooks.ts` | 對話 Hooks | 150 |
| `src/hooks/tool.hooks.ts` | 工具 Hooks | 100 |

#### 3.1.3 Hook 實現順序

1. **`chat.message`** - 消息捕獲（最基礎）
2. **`tool.execute.before/after`** - 工具追蹤
3. **`experimental.session.compacting`** - 壓縮控制
4. **`experimental.chat.messages.transform`** - 上下文注入
5. **`experimental.chat.system.transform`** - System Prompt 變換

#### 3.1.4 交付物

- [ ] `opencode-memory-plugin` 初始版本
- [ ] 基本記憶池實現
- [ ] SQLite 持久化
- [ ] 通過 `chat.message` 捕獲消息
- [ ] 通過 `tool.execute.*` 追蹤工具

### Phase 2: Memory Plugin 高級功能 (3-4 週)

#### 3.2.1 4層壓縮系統

對齊 CrushCL 的 4 層壓縮：

```typescript
// L1: 微壓縮 (<1ms)
async microCompact(messages: Message[]): Promise<void> {
    if (messages.length > 20) {
        await this.foldToolResults(messages);
    }
}

// L2: 自動壓縮 (~100ms)
async autoCompact(budget: number): Promise<void> {
    if (this.getTokenUsage() >= budget * 0.85) {
        await this.summarize();
    }
}

// L3: 完整壓縮 (5-30s)
async fullCompact(): Promise<void> {
    await this.forkSummarize();
}

// L4: 會話記憶
async sessionMemory(): Promise<void> {
    await this.saveToPersistent();
}
```

#### 3.2.2 檢索增強

- 向量相似度檢索
- 混合檢索策略
- 相關性反饋

#### 3.2.3 交付物

- [ ] 4層壓縮完整實現
- [ ] 檢索系統
- [ ] 與 CrushCL L1-L4 對齊

### Phase 3: Agent Config Plugin (3-4 週)

#### 3.3.1 項目初始化

```bash
mkdir -p opencode-agent-config-plugin
cd opencode-agent-config-plugin
npm init -y
npm install @opencode-ai/plugin typescript js-yaml
```

#### 3.3.2 核心組件

| 檔案 | 職責 |
|------|------|
| `src/config/manager.ts` | 配置管理器 |
| `src/persona/registry.ts` | Persona 註冊表 |
| `src/persona/builder.ts` | Prompt 構建器 |
| `src/behaviors/engine.ts` | 行為引擎 |

#### 3.3.3 內建 Persona

- [ ] `architect` - 系統架構師
- [ ] `coder` - 代碼實現專家
- [ ] `reviewer` - 代碼審查專家
- [ ] `researcher` - 研究分析專家

### Phase 4: 整合與遷移 (2-3 週)

#### 3.4.1 現有 Skill 遷移

| Skill | 遷移目標 | 狀態 |
|-------|---------|------|
| `clawcode-session-memory` | MemoryPlugin | 廢除原 Skill |
| `enhanced-memory` | MemoryPlugin | 廢除原 Skill |
| `oh-my-opencode.json` | AgentConfigPlugin | 自動適配 |

#### 3.4.2 配置更新

```json
// opencode.json
{
  "plugin": [
    "opencode-memory-plugin@latest",
    "opencode-agent-config-plugin@latest"
  ]
}
```

## 4. 技術規格

### 4.1 技術棧

| 組件 | 技術 |
|------|------|
| 語言 | TypeScript 5.x |
| 運行時 | Node.js 20+ |
| 構建 | tsdx / rollup |
| 測試 | Vitest |
| 持久化 | better-sqlite3 |

### 4.2 依賴

```json
{
  "dependencies": {
    "@opencode-ai/plugin": "^1.1.4",
    "better-sqlite3": "^9.0.0"
  },
  "devDependencies": {
    "typescript": "^5.3.0",
    "vitest": "^1.0.0",
    "@types/better-sqlite3": "^7.6.0"
  }
}
```

### 4.3 類型定義

```typescript
// src/types/index.ts
export interface MemoryEntry {
    id: string;
    sessionID: string;
    type: 'message' | 'tool' | 'result' | 'summary';
    content: string;
    tokens: number;
    timestamp: number;
    metadata?: Record<string, any>;
}

export interface MemoryPool {
    hot: MemoryEntry[];      // 最近 N 條
    warm: MemoryEntry[];     // 工具結果
    cold: MemoryEntry[];      // 壓縮摘要
}

export interface Persona {
    id: string;
    name: string;
    systemPrompt: string;
    tools: string[];
    parameters: Record<string, any>;
    behaviors: Behavior[];
}
```

## 5. 測試策略

### 5.1 單元測試

```typescript
// test/memory-pool.test.ts
describe('MemoryPool', () => {
    it('should add entries', () => {
        const pool = new MemoryPool();
        pool.add({ type: 'message', content: 'test' });
        expect(pool.size()).toBe(1);
    });
    
    it('should trigger compaction at threshold', () => {
        const pool = new MemoryPool({ threshold: 0.85 });
        // ... test compaction trigger
    });
});
```

### 5.2 集成測試

```typescript
// test/hooks-integration.test.ts
describe('Hook Integration', () => {
    it('should capture chat messages', async () => {
        const hook = chatMessageHook(memorySystem);
        await hook({ sessionID: 'test' }, { message: 'test', parts: [] });
        expect(memorySystem.pools.get('test').size()).toBe(1);
    });
});
```

## 6. 風險與緩解

| 風險 | 影響 | 緩解措施 |
|------|------|---------|
| Hook 性能開銷 | 高 | 非同步處理，緩存結果 |
| SQLite 鎖競爭 | 中 | 使用 WAL 模式，連接池 |
| 配置向後相容 | 高 | 完整適配層，嚴格測試 |
| 記憶體洩漏 | 高 | 定期清理，TTL 机制 |

## 7. 資源估算

| 階段 | 人力 | 時間 | 複雜度 |
|------|------|------|--------|
| Phase 1 | 1 人 | 4-6 週 | 中 |
| Phase 2 | 1 人 | 3-4 週 | 高 |
| Phase 3 | 1 人 | 3-4 週 | 中 |
| Phase 4 | 1 人 | 2-3 週 | 低 |
| **總計** | **1 人** | **12-17 週** | - |

## 8. 里程碑

| 里程碑 | 日期 | 交付物 |
|--------|------|--------|
| M1 | +4 週 | Memory Plugin 核心功能 |
| M2 | +8 週 | 4層壓縮 + 檢索 |
| M3 | +12 週 | Agent Config Plugin |
| M4 | +15 週 | 完整集成 + 遷移 |

## 9. 結論

這個實作計劃將：

1. **充分發揮 OpenCode Plugin Hooks** - 深度整合對話生命週期
2. **對齊 CrushCL 先進架構** - 借鑒其 4 層壓縮和 Guardian 機制
3. **實現真正的自動化** - 無需用戶手動觸發記憶體操作
4. **保持向後相容** - 現有 Skill 和配置平滑遷移

通過 Plugin 架構，我們將記憶體管理和 Agent 配置從「外掛」提升為「內核功能」。
