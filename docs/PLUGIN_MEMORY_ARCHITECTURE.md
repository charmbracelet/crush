# OpenCode Memory Management Plugin 架構設計

## 1. 概述

### 1.1 設計目標

將 `clawcode-session-memory`、`enhanced-memory`、`memory-mcp` 等記憶體相關 Skill 重構為 Plugin，充分利用 OpenCode Plugin Hooks 系統，實現：

- **深度整合**：攔截對話生命週期各階段
- **自動化**：無需用戶手動觸發
- **持久化**：跨會話記憶保持
- **自適應**：根據上下文智能觸發壓縮/檢索

### 1.2 當前 Skill 架構問題

| Skill | 問題 |
|-------|------|
| `clawcode-session-memory` | 被動加載，無法攔截壓縮時機 |
| `enhanced-memory` | 獨立運行，與 OpenCode 上下文隔離 |
| `memory-mcp` | MCP 協議限制，無法訪問內部狀態 |

## 2. Plugin 介面分析

### 2.1 OpenCode Plugin Hooks

```typescript
// 完整 Hooks 介面
interface Hooks {
    // 事件
    event?: (input: { event: Event }) => Promise<void>;
    config?: (input: Config) => Promise<void>;
    
    // 工具
    tool?: { [key: string]: ToolDefinition };
    "tool.execute.before"?: (input, output) => Promise<void>;
    "tool.execute.after"?: (input, output) => Promise<void>;
    "tool.definition"?: (input, output) => Promise<void>;
    
    // 對話
    "chat.message"?: (input, output) => Promise<void>;
    "chat.params"?: (input, output) => Promise<void>;
    "chat.headers"?: (input, output) => Promise<void>;
    
    // 許可
    "permission.ask"?: (input, output) => Promise<void>;
    
    // 命令
    "command.execute.before"?: (input, output) => Promise<void>;
    "shell.env"?: (input, output) => Promise<void>;
    
    // 實驗性
    "experimental.session.compacting"?: (input, output) => Promise<void>;
    "experimental.chat.messages.transform"?: (input, output) => Promise<void>;
    "experimental.chat.system.transform"?: (input, output) => Promise<void>;
    "experimental.text.complete"?: (input, output) => Promise<void>;
    
    // 認證
    auth?: AuthHook;
    provider?: ProviderHook;
}
```

### 2.2 關鍵 Hook 分析

| Hook | 記憶體應用場景 |
|------|---------------|
| `chat.message` | 捕捉每條消息，構建記憶池 |
| `chat.params` | 調整 LLM 參數（如增加 max_tokens） |
| `tool.execute.before` | 記錄工具調用意圖 |
| `tool.execute.after` | 記錄工具執行結果，更新記憶 |
| `session.compacting` | **核心**：自定義壓縮邏輯 |
| `messages.transform` | 對即將發送的消息進行變換 |
| `system.transform` | 修改系統提示詞 |

## 3. 記憶體 Plugin 架構

### 3.1 模組結構

```
opencode-memory-plugin/
├── src/
│   ├── index.ts              # Plugin 入口
│   ├── hooks/
│   │   ├── chat.hooks.ts     # chat.message, chat.params
│   │   ├── tool.hooks.ts     # tool.execute.before/after
│   │   ├── compaction.hooks.ts # session.compacting
│   │   └── transform.hooks.ts # messages/system transform
│   ├── memory/
│   │   ├── memory-pool.ts    # 記憶池實現
│   │   ├── compact.ts        # 壓縮邏輯
│   │   ├── retrieval.ts       # 檢索邏輯
│   │   └── semantic.ts       # 語義分析
│   ├── storage/
│   │   ├── sqlite.ts         # SQLite 持久化
│   │   └── cache.ts          # 內存緩存
│   └── types/
│       └── index.ts          # 類型定義
├── package.json
├── tsconfig.json
└── README.md
```

### 3.2 核心功能映射

#### A. 會話記憶池 (Session Memory Pool)

```typescript
// 利用 chat.message Hook
export async function chatMessageHook(
    input: { sessionID: string; messageID?: string },
    output: { message: UserMessage; parts: Part[] }
) {
    // 1. 捕捉每條消息
    const memoryPool = getSessionMemoryPool(input.sessionID);
    
    // 2. 提取關鍵資訊
    const entity = extractEntities(output.message);
    const relations = extractRelations(output.message);
    
    // 3. 更新記憶池
    await memoryPool.add({
        type: 'message',
        content: output.message.content,
        entities,
        relations,
        timestamp: Date.now()
    });
    
    // 4. 觸發檢索（如果需要）
    if (shouldRetrieve(memoryPool)) {
        const context = await memoryPool.retrieve(output.message);
        attachContextToParts(output.parts, context);
    }
}
```

#### B. 自定義壓縮邏輯

```typescript
// 利用 session.compacting Hook
export async function sessionCompactingHook(
    input: { sessionID: string },
    output: { context: string[]; prompt?: string }
) {
    const memoryPool = getSessionMemoryPool(input.sessionID);
    
    // 1. 分析當前上下文
    const analysis = await memoryPool.analyze();
    
    // 2. 生成壓縮提示
    if (analysis.compressionNeeded) {
        output.prompt = buildCompactionPrompt(analysis);
    }
    
    // 3. 添加相關上下文
    output.context.push(...await memoryPool.getRelevantContext());
}
```

#### C. 工具執行追蹤

```typescript
// 利用 tool.execute.before / after Hooks
export async function toolExecuteBeforeHook(
    input: { tool: string; sessionID: string; callID: string },
    output: { args: any }
) {
    const tracker = getToolTracker(input.sessionID);
    tracker.recordIntent(input.tool, output.args);
}

export async function toolExecuteAfterHook(
    input: { tool: string; sessionID: string; callID: string; args: any },
    output: { title: string; output: string; metadata: any }
) {
    const tracker = getToolTracker(input.sessionID);
    tracker.recordResult(input.tool, output);
    
    // 更新記憶池
    const memoryPool = getSessionMemoryPool(input.sessionID);
    await memoryPool.addToolResult(input.tool, output);
}
```

### 3.3 記憶體層級設計

```typescript
// 記憶體分層
interface MemoryLayer {
    // L1: 熱記憶 - 最近 N 條消息
    hot: HotMemoryLayer;      // ~20 messages, instant access
    
    // L2: 溫記憶 - 工具結果摘要
    warm: WarmMemoryLayer;     // tool results, 5min TTL
    
    // L3: 冷記憶 - 壓縮後的會話摘要
    cold: ColdMemoryLayer;     // compressed summary, persistent
    
    // L4: 持久記憶 - 跨會話項目知識
    persistent: PersistentLayer; // project knowledge, SQLite
}

// 觸發條件
const MEMORY_TRIGGERS = {
    hot: { count: 20 },
    warm: { ttl: 5 * 60 * 1000 },  // 5 minutes
    cold: { tokens: 0.85 * MAX_TOKENS },
    persistent: { sessionEnd: true }
};
```

## 4. 與 CrushCL 整合

### 4.1 CrushCL 現有組件對應

| CrushCL 組件 | Plugin 對應 | Hook |
|--------------|------------|------|
| `kernel/context/compactor.go` | `session.compacting` | 壓縮觸發 |
| `kernel/context/memory_pool.go` | `MemoryPool` | chat.message |
| `kernel/usage_tracker.go` | `TokenTracker` | chat.params |
| `kernel/hook_pipeline.go` | `TransformHooks` | messages/system transform |
| `agent/guardian/` | `HealthMonitor` | tool.execute.before/after |

### 4.2 借鑒 CrushCL 的實現

```typescript
// 參考 CrushCL CompressionOrchestrator
class MemoryOrchestrator {
    private layers: MemoryLayer[];
    private hooks: HookRegistration;
    
    // L1: 即時微壓縮 (<1ms)
    async microCompact(messages: Message[]): Promise<void> {
        if (messages.length > 20) {
            // 折疊早期工具結果
            await this.foldToolResults(messages);
        }
    }
    
    // L2: 自動壓縮 (~100ms)
    async autoCompact(tokenBudget: number): Promise<void> {
        if (this.getTokenUsage() >= tokenBudget * 0.85) {
            await this.triggerSummarization();
        }
    }
    
    // L3: 完整壓縮 (5-30s)
    async fullCompact(): Promise<void> {
        await this.forkSummarize();
    }
    
    // L4: 會話記憶
    async sessionMemory(): Promise<void> {
        await this.saveToPersistentLayer();
    }
}
```

## 5. Plugin 實現

### 5.1 主入口

```typescript
// index.ts
import type { Plugin, PluginInput, Hooks } from '@opencode-ai/plugin';

export default async function memoryPlugin(
    input: PluginInput,
    options?: PluginOptions
): Promise<Hooks> {
    // 初始化記憶體系統
    const memorySystem = await initMemorySystem(input, options);
    
    return {
        // 對話鉤子
        'chat.message': chatMessageHook(memorySystem),
        'chat.params': chatParamsHook(memorySystem),
        'chat.headers': chatHeadersHook(memorySystem),
        
        // 工具鉤子
        'tool.execute.before': toolExecuteBeforeHook(memorySystem),
        'tool.execute.after': toolExecuteAfterHook(memorySystem),
        'tool.definition': toolDefinitionHook(memorySystem),
        
        // 壓縮鉤子
        'experimental.session.compacting': sessionCompactingHook(memorySystem),
        
        // 轉換鉤子
        'experimental.chat.messages.transform': messagesTransformHook(memorySystem),
        'experimental.chat.system.transform': systemTransformHook(memorySystem),
        'experimental.text.complete': textCompleteHook(memorySystem),
        
        // 事件鉤子
        'event': eventHook(memorySystem),
        'config': configHook(memorySystem),
    };
}

async function initMemorySystem(input: PluginInput, options?: PluginOptions) {
    const db = await openDatabase(`${input.directory}/.opencode/memory.db`);
    const cache = new LRUCache({ max: 1000 });
    
    return {
        input,
        options,
        db,
        cache,
        pools: new Map(),  // sessionID -> MemoryPool
    };
}
```

### 5.2 配置 Schema

```typescript
// 支援的配置選項
const DEFAULT_OPTIONS: PluginOptions = {
    // 記憶體層級
    layers: {
        hot: { maxMessages: 20 },
        warm: { ttlMs: 5 * 60 * 1000 },
        cold: { tokenThreshold: 0.85 },
        persistent: { enabled: true }
    },
    
    // 壓縮設定
    compaction: {
        enabled: true,
        triggers: {
            auto: true,
            manual: true,
            tokenThreshold: 0.85
        }
    },
    
    // 檢索設定
    retrieval: {
        enabled: true,
        topK: 5,
        similarityThreshold: 0.7
    },
    
    // 追蹤設定
    tracking: {
        tools: true,
        messages: true,
        entities: true,
        relations: true
    }
};
```

## 6. 遷移策略

### 6.1 從 Skill 到 Plugin

| Skill | Plugin 組件 | 遷移優先級 |
|-------|------------|----------|
| `clawcode-session-memory` | `MemoryPool` + `HotLayer` | P0 |
| `enhanced-memory` | `Retrieval` + `ColdLayer` | P0 |
| `memory-mcp` | 廢除（Plugin 直接集成） | P1 |
| `clawcode-runtime-patterns` | `Hooks` 實現 | P1 |

### 6.2 遷移步驟

1. **Phase 1: 核心實現**
   - 實現 `MemoryPool` 類
   - 實現 `chat.message` Hook
   - 實現 `tool.execute.before/after` Hooks

2. **Phase 2: 壓縮整合**
   - 實現 `session.compacting` Hook
   - 實現 `messages.transform` Hook
   - 對齊 CrushCL L1-L4 觸發邏輯

3. **Phase 3: 持久化**
   - 實現 SQLite 存儲
   - 實現跨會話檢索
   - 實現項目知識庫

4. **Phase 4: 優化**
   - 性能優化
   - 緩存策略
   - 監控和日誌

## 7. 預期效果

### 7.1 功能對比

| 功能 | Skill 實現 | Plugin 實現 |
|------|-----------|------------|
| 消息捕捉 | 被動，需手動觸發 | 主動，自動捕捉 |
| 工具追蹤 | 有限 | 完整生命週期 |
| 壓縮時機 | 外部觸發 | 精確在 `session.compacting` |
| 上下文注入 | 手動拼接 | 自動在 `messages.transform` |
| 跨會話記憶 | 需另外實現 | 內建 `PersistentLayer` |

### 7.2 性能預期

- **L1 微壓縮**: < 1ms（記憶體操作）
- **L2 自動壓縮**: ~100ms（本地計算）
- **L3 完整壓縮**: 5-30s（取決於上下文大小）
- **L4 持久化**: < 100ms（SQLite 寫入）

## 8. 結論

通過將記憶體相關 Skill 重構為 Plugin，可以：

1. **充分利用 OpenCode 內部 Hooks**，實現更深度的整合
2. **借鑒 CrushCL 的 4 層壓縮架構**，實現精確的觸發控制
3. **消除 MCP 協議限制**，直接訪問內部狀態
4. **實現真正的自動化**，無需用戶手動干預

這個架構將記憶體管理從「外掛」提升為「內核功能」。
