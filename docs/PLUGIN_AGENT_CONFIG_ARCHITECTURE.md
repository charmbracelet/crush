# OpenCode Persona/Agent Configuration Plugin 架構設計

## 1. 概述

### 1.1 設計目標

將 `oh-my-opencode.json` 的靜態 agent 配置擴展為 Plugin 驅動的動態系統，實現：

- **動態 Persona 載入**：根據上下文自動切換行為
- **行為鉤子**：對 agent 的輸入輸出進行變換
- **工具存取控制**：細粒度的 tool 權限管理
- **自定義 System Prompt**：每個 agent 可有自己的系統提示
- **學習適應**：基於歷史互動調整行為

### 1.2 當前架構限制

```json
// oh-my-opencode.json - 僅支援 model 選擇
{
  "agents": {
    "sisyphus": { "model": "minimax/MiniMax-M2.7-highspeed" }
  }
}
```

**問題**：
- 無法自定義 agent 行為
- 無法動態調整工具集
- 無法注入上下文相關的 system prompt
- 無法根據任務類型自適應

## 2. Plugin 架構

### 2.1 核心概念

```
┌─────────────────────────────────────────────────────────────┐
│                    Agent Configuration Plugin                │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Persona   │  │   Behavior  │  │   Context   │       │
│  │   Loader    │  │    Hooks    │  │   Builder    │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ▼                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Agent Configuration Manager             │   │
│  │  - 動態載入 Persona                                   │   │
│  │  - 合併 Hooks                                        │   │
│  │  │- 構建 Context                                    │   │
│  └─────────────────────────────────────────────────────┘   │
│                          │                                   │
│                          ▼                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              OpenCode Agent Runtime                  │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Plugin 介面擴展

```typescript
// 新的 Agent Configuration Hooks
interface AgentConfigHooks {
    // 獲取 agent 配置
    'agent.config'?: (input: {
        agentID: string;
        task?: string;
        context?: any;
    }, output: {
        model: string;
        systemPrompt?: string;
        tools?: string[];
        parameters?: Record<string, any>;
    }) => Promise<void>;
    
    // 載入 persona
    'agent.persona'?: (input: {
        agentID: string;
        task?: string;
    }, output: {
        name: string;
        description?: string;
        avatar?: string;
        systemPrompt: string;
        behaviors: Behavior[];
    }) => Promise<void>;
    
    // 行為鉤子
    'agent.behavior'?: (input: {
        agentID: string;
        phase: 'thinking' | 'acting' | 'responding';
    }, output: {
        intercept: boolean;
        transform?: (input: any) => any;
    }) => Promise<void>;
    
    // 上下文構建
    'agent.context'?: (input: {
        agentID: string;
        sessionID: string;
        task: string;
    }, output: {
        systemPrompt: string[];
        context: string[];
        examples?: Example[];
    }) => Promise<void>;
}
```

### 2.3 Persona 結構

```typescript
// persona.ts
interface Persona {
    id: string;
    name: string;
    description: string;
    avatar?: string;
    
    // 系統提示
    systemPrompt: SystemPrompt;
    
    // 行為配置
    behaviors: Behavior[];
    
    // 工具配置
    tools: ToolConfig;
    
    // 參數配置
    parameters: AgentParameters;
    
    // 適應規則
    adaptation?: AdaptationRule[];
}

interface SystemPrompt {
    base: string;
    templates: Record<string, string>;  // task-type -> prompt
    dynamic?: DynamicPromptBuilder[];
}

interface Behavior {
    id: string;
    trigger: BehaviorTrigger;
    action: BehaviorAction;
}

interface BehaviorTrigger {
    type: 'task' | 'context' | 'time' | 'tool' | 'error';
    condition: string;  // DSL 或錨點表達式
}

interface BehaviorAction {
    type: 'inject' | 'modify' | 'block' | 'escalate';
    config: Record<string, any>;
}

interface ToolConfig {
    allowed: string[];
    denied: string[];
    required?: string[];
    parameters?: Record<string, ToolParameterConfig>;
}

interface AdaptationRule {
    trigger: string;  // 觀察到的模式
    action: 'increase' | 'decrease' | 'add' | 'remove';
    target: 'temperature' | 'tools' | 'prompt';
    adjustment: any;
}
```

## 3. 實現設計

### 3.1 Plugin 結構

```
opencode-agent-config-plugin/
├── src/
│   ├── index.ts              # Plugin 入口
│   ├── config/
│   │   ├── manager.ts        # 配置管理器
│   │   ├── loader.ts         # Persona 載入器
│   │   └── merger.ts         # 配置合併
│   ├── persona/
│   │   ├── registry.ts       # Persona 註冊表
│   │   ├── builder.ts        # 動態 Prompt 構建
│   │   └── adapter.ts        # 向後相容適配器
│   ├── hooks/
│   │   ├── agent.hooks.ts    # agent.config, agent.persona
│   │   ├── behavior.hooks.ts  # agent.behavior
│   │   └── context.hooks.ts   # agent.context
│   ├── behaviors/
│   │   ├── engine.ts         # 行為引擎
│   │   ├── triggers.ts       # 觸發器
│   │   └── actions.ts         # 動作
│   ├── adaptation/
│   │   ├── learner.ts        # 學習器
│   │   └── optimizer.ts       # 參數優化
│   └── storage/
│       └── sqlite.ts         # 配置持久化
├── personas/                  # 內建 Persona
│   ├── architect.yaml
│   ├── coder.yaml
│   ├── reviewer.yaml
│   └── researcher.yaml
├── package.json
└── tsconfig.json
```

### 3.2 核心組件

#### A. 配置管理器

```typescript
// config/manager.ts
class AgentConfigManager {
    private personas: Map<string, Persona>;
    private behaviors: Map<string, Behavior[]>;
    private adaptations: Map<string, AdaptationRule[]>;
    
    async getConfig(agentID: string, task?: string): Promise<AgentConfig> {
        const persona = this.personas.get(agentID);
        if (!persona) {
            return this.getDefaultConfig();
        }
        
        // 1. 獲取基礎配置
        let config = this.buildBaseConfig(persona);
        
        // 2. 應用行為規則
        config = await this.applyBehaviors(config, task);
        
        // 3. 構建上下文
        config = await this.buildContext(config, task);
        
        // 4. 應用適應規則
        config = await this.applyAdaptations(config, agentID);
        
        return config;
    }
    
    private buildBaseConfig(persona: Persona): AgentConfig {
        return {
            model: persona.parameters.model,
            systemPrompt: this.buildSystemPrompt(persona),
            tools: this.resolveTools(persona.tools),
            temperature: persona.parameters.temperature,
            topP: persona.parameters.topP,
            maxTokens: persona.parameters.maxTokens,
        };
    }
    
    private buildSystemPrompt(persona: Persona): string {
        const parts: string[] = [];
        
        // Base prompt
        parts.push(persona.systemPrompt.base);
        
        // Dynamic templates
        for (const [key, template] of Object.entries(persona.systemPrompt.templates)) {
            parts.push(`\n\n## ${key} Context\n${template}`);
        }
        
        // Dynamic builders
        if (persona.systemPrompt.dynamic) {
            for (const builder of persona.systemPrompt.dynamic) {
                parts.push(builder.build());
            }
        }
        
        return parts.join('\n');
    }
}
```

#### B. 行為引擎

```typescript
// behaviors/engine.ts
class BehaviorEngine {
    async evaluate(
        behaviors: Behavior[],
        context: ExecutionContext
    ): Promise<BehaviorResult[]> {
        const results: BehaviorResult[] = [];
        
        for (const behavior of behaviors) {
            const triggered = await this.isTriggered(behavior.trigger, context);
            if (triggered) {
                const result = await this.execute(behavior.action, context);
                results.push(result);
                
                if (behavior.action.type === 'block') {
                    break;  // 阻止後續行為
                }
            }
        }
        
        return results;
    }
    
    private async isTriggered(
        trigger: BehaviorTrigger,
        context: ExecutionContext
    ): Promise<boolean> {
        switch (trigger.type) {
            case 'task':
                return this.matchTask(trigger.condition, context.task);
            case 'tool':
                return this.matchTool(trigger.condition, context.tool);
            case 'error':
                return this.matchError(trigger.condition, context.error);
            case 'time':
                return this.matchTime(trigger.condition, context.timestamp);
            default:
                return false;
        }
    }
    
    private async execute(
        action: BehaviorAction,
        context: ExecutionContext
    ): Promise<BehaviorResult> {
        switch (action.type) {
            case 'inject':
                return this.inject(action.config, context);
            case 'modify':
                return this.modify(action.config, context);
            case 'block':
                return this.block(action.config, context);
            case 'escalate':
                return this.escalate(action.config, context);
        }
    }
}
```

#### C. Persona 註冊表

```typescript
// persona/registry.ts
class PersonaRegistry {
    private personas: Map<string, Persona> = new Map();
    
    register(persona: Persona): void {
        this.validate(persona);
        this.personas.set(persona.id, persona);
    }
    
    async loadFromPlugin(pluginPath: string): Promise<void> {
        const plugin = await import(pluginPath);
        const personas = await plugin.getPersonas?.();
        if (personas) {
            for (const persona of personas) {
                this.register(persona);
            }
        }
    }
    
    async loadFromYAML(yamlPath: string): Promise<void> {
        const yaml = await import('yaml');
        const content = await readFile(yamlPath, 'utf-8');
        const persona = yaml.parse(content);
        this.register(persona);
    }
    
    get(id: string): Persona | undefined {
        return this.personas.get(id);
    }
    
    match(task: string): Persona[] {
        // 根據任務類型匹配最佳 persona
        return Array.from(this.personas.values())
            .filter(p => this.isRelevant(p, task))
            .sort((a, b) => this.score(a, task) - this.score(b, task));
    }
    
    private isRelevant(persona: Persona, task: string): boolean {
        // 檢查 persona 的適用範圍
        return true;  // TODO: 實現邏輯
    }
}
```

### 3.3 向後相容

```typescript
// oh-my-opencode.json 轉換為 Persona
function adaptLegacyConfig(config: LegacyAgentConfig): Persona {
    return {
        id: config.agentID,
        name: config.agentID,
        description: `Legacy agent: ${config.agentID}`,
        systemPrompt: {
            base: `You are ${config.agentID}.`,
            templates: {},
            dynamic: []
        },
        behaviors: [],
        tools: {
            allowed: ['*'],  // 繼承原有行爲
            denied: []
        },
        parameters: {
            model: config.model,
            temperature: 0.7,
            topP: 1.0,
            maxTokens: 8192
        }
    };
}
```

## 4. 內建 Persona 定義

### 4.1 Architect Persona

```yaml
# personas/architect.yaml
id: architect
name: Architect
description: System architect specializing in design patterns and architecture

systemPrompt:
  base: |
    You are an elite software architect with deep expertise in:
    - System design patterns (DDD, Event Sourcing, CQRS, etc.)
    - API design and microservices architecture
    - Database schema design and optimization
    - Scalability and performance engineering
    
    When designing systems:
    1. Start with requirements analysis
    2. Identify bounded contexts
    3. Define API contracts
    4. Plan data models
    5. Consider cross-cutting concerns
    
  templates:
    api-design: |
      ## API Design Principles
      - Use RESTful conventions
      - Version APIs from day one
      - Design for failure
      - Implement rate limiting
    db-design: |
      ## Database Design Guidelines
      - Normalize for integrity
      - Denormalize for performance
      - Index strategically
      - Plan for migration

behaviors:
  - id: suggest-architecture
    trigger:
      type: task
      condition: contains_any[task, "design", "architecture", "system"]
    action:
      type: inject
      config:
        prompt: "\n\n## Architecture Review Required\nBefore proceeding, consider:\n1. Scalability requirements\n2. Data consistency needs\n3. Service boundaries"

tools:
  allowed:
    - read
    - grep
    - glob
    - bash
    - edit
  denied:
    - deploy
    - destroy

parameters:
  model: minimax/MiniMax-M2.7-highspeed
  temperature: 0.6
  maxTokens: 8192
```

### 4.2 Coder Persona

```yaml
# personas/coder.yaml
id: coder
name: Coder
description: Implementation specialist focused on clean, efficient code

systemPrompt:
  base: |
    You are a pragmatic coder who:
    - Writes clean, maintainable code
    - Follows SOLID principles
    - Writes tests alongside code
    - Documents complex logic
    
    Implementation workflow:
    1. Understand requirements
    2. Plan structure
    3. Implement incrementally
    4. Test as you go
    5. Refactor for clarity

behaviors:
  - id: suggest-tests
    trigger:
      type: tool
      condition: equals[tool, "edit"]
    action:
      type: inject
      config:
        prompt: "\n\n## Testing Checklist\n- Unit tests written?\n- Edge cases covered?\n- Error handling tested?"

tools:
  allowed:
    - read
    - grep
    - glob
    - bash
    - edit
    - write
    - view
    - test

parameters:
  model: minimax/MiniMax-M2.7-highspeed
  temperature: 0.5
  maxTokens: 16384
```

## 5. Plugin 配置

```json
{
  "opencode": {
    "plugin": [
      "opencode-agent-config-plugin@latest"
    ]
  },
  "agentConfig": {
    "plugin": {
      "enabled": true,
      "defaultPersona": "coder",
      "personaDirectory": "./personas",
      "enableAdaptation": true,
      "adaptationHistory": 100
    }
  }
}
```

## 6. 遷移策略

### 6.1 階段規劃

| 階段 | 內容 | 優先級 |
|------|------|--------|
| Phase 1 | Plugin 框架 + 基本 Persona 載入 | P0 |
| Phase 2 | 行為引擎 + 觸發器 | P1 |
| Phase 3 | 向後相容適配器 | P1 |
| Phase 4 | 自適應學習系統 | P2 |
| Phase 5 | YAML Persona 支援 | P2 |

### 6.2 向後相容

- 現有 `oh-my-opencode.json` 自動轉換為預設 Persona
- 現有 agent 行為不變
- 新功能可選啟用

## 7. 預期效果

| 功能 | 當前 | Plugin 實現 |
|------|------|------------|
| Agent 配置 | 靜態 JSON | 動態 Persona |
| System Prompt | 單一固定 | 模板 + 動態 |
| 工具控制 | 全域設置 | Per-Persona |
| 行為觸發 | 無 | 規則引擎 |
| 自適應 | 無 | 學習優化 |

## 8. 結論

通過 Plugin 架構，實現：

1. **動態 Persona 系統**：根據任務自動切換最佳配置
2. **行為引擎**：可編程的干預和變換規則
3. **自適應學習**：基於歷史優化參數
4. **向後相容**：現有配置自動適配
