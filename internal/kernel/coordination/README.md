# Coordination Module

## 概述

Coordination 模組是 CrushCL 的核心協調系統，負責：
1. **任務調度** - 優先級隊列管理
2. **資源管理** - CPU、Memory、Token、Budget 追蹤
3. **負載均衡** - CL/Claude Code 執行者選擇

## 組件

### 1. TaskScheduler (task_scheduler.go)

任務調度器，實現優先級隊列和任務管理。

```go
scheduler := coordination.NewTaskScheduler()
scheduler.Submit(&coordination.Task{
    Prompt:   "Build user authentication",
    Priority: coordination.PriorityHigh,
})
```

**特性**：
- 基於堆的優先級隊列 (O(log n) 插入/彈出)
- 最大並發任務控制
- 任務超時管理
- 任務歷史追蹤

### 2. ResourceManager (resource_manager.go)

資源管理器，追蹤和管理系統資源。

```go
rm := coordination.NewResourceManager()

// 獲取資源
resID, err := rm.Acquire(ctx, map[coordination.ResourceType]float64{
    coordination.ResourceCPU: 1,
})

// 釋放資源
rm.Release(resID)
```

**資源類型**：
- `ResourceCPU` - 併發任務數
- `ResourceMemory` - 記憶體使用
- `ResourceToken` - Token 預算
- `ResourceBudget` - 金錢預算

### 3. LoadBalancer (load_balancer.go)

負載均衡器，選擇最佳執行者。

```go
lb := coordination.NewLoadBalancer()

// 選擇執行者
executor := lb.SelectExecutor(ctx, coordination.SelectOptions{
    TaskComplexity: 0.7,
    MaxCost:        0.05,
})
```

**執行者**：
- `ExecutorCL` - CrushCL Native
- `ExecutorClaudeCode` - Claude Code CLI
- `ExecutorHybrid` - 混合模式

**策略**：
- `StrategyRoundRobin` - 輪詢
- `StrategyLeastLoad` - 最小負載
- `StrategyWeighted` - 權重選擇
- `StrategyCostOptimized` - 成本優化
- `StrategyAdaptive` - 自適應（預設）

### 4. Coordinator (coordinator.go)

協調器，整合所有模組。

```go
coord := coordination.NewCoordinator()

// 提交任務
task, _ := coord.SubmitTask("Build feature X", coordination.PriorityNormal, []string{"Read", "Write"})

// 完成任務
coord.CompleteTask(task.ID, &coordination.TaskResult{
    Output:   "Feature built",
    Tokens:   100,
    CostUSD:  0.01,
    Duration: 1 * time.Second,
})
```

## 使用範例

```go
package main

import (
    "context"
    "time"
    
    "github.com/charmbracelet/crushcl/internal/kernel/coordination"
)

func main() {
    // 創建協調器
    coord := coordination.NewCoordinator()
    
    // 提交多個任務
    tasks := []struct {
        prompt   string
        priority coordination.TaskPriority
    }{
        {"Build login feature", coordination.PriorityHigh},
        {"Add user profile", coordination.PriorityNormal},
        {"Update docs", coordination.PriorityLow},
    }
    
    for _, t := range tasks {
        coord.SubmitTask(t.prompt, t.priority, []string{"Read", "Write"})
    }
    
    // 獲取統計
    stats := coord.GetStats()
    println(stats)
    
    // 關閉
    coord.Shutdown()
}
```

## 配置

```go
config := coordination.CoordinatorConfig{
    Scheduler: coordination.SchedulerConfig{
        MaxConcurrentTasks: 10,
        TaskTimeout:        5 * time.Minute,
    },
    ResourceManager: coordination.ResourceManagerConfig{
        MaxConcurrentTasks: 10,
        MaxTokenBudget:    200000,
        MaxBudgetUSD:      10.0,
        WarningThreshold:  0.70,
    },
    LoadBalancer: coordination.LoadBalancerConfig{
        DefaultStrategy: coordination.StrategyAdaptive,
    },
}

coord := coordination.NewCoordinatorWithConfig(config)
```

## 狀態追蹤

```go
// 獲取協調器狀態
state := coord.GetState()
fmt.Printf("Processed: %d, Failed: %d, Cost: $%.4f\n",
    state.TasksProcessed, state.TasksFailed, state.TotalCostUSD)

// 獲取完整統計
stats := coord.GetStats()
```

## 整合

### 與 HybridBrain 整合

```go
brain := NewHybridBrain(config)

// 在 HybridBrain 中使用 Coordinator
coord := coordination.NewCoordinator()

result := coord.SubmitTask(task, priority, tools)
// 協調器會選擇執行者並管理資源
```

### 與 HTTP Server 整合

```go
server := server.NewServer()
coord := coordination.NewCoordinator()

// API 端點可以使用 coord 進行任務管理
```
