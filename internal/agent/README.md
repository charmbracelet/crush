# Agent 溝通框架與防卡住機制

## 概述

這是 CrushCL 的增強版 Agent 溝通框架，提供：

1. **Message Bus** - Agent 間消息傳遞
2. **State Machine** - Agent 狀態管理
3. **Guardian** - 防卡住/心跳/熔斷機制
4. **SwarmExt** - 整合以上所有功能

## 目錄結構

```
internal/agent/
├── swarm.go              # 原有 Swarm（保持向後兼容）
├── swarm_ext.go          # 增強版 Swarm（整合新框架）
├── messagebus/
│   └── messagebus.go     # 消息總線
├── statmachine/
│   └── state_machine.go  # 狀態機
└── guardian/
    └── guardian.go       # 防卡住守護者
```

## 組件說明

### 1. Message Bus（消息總線）

**功能**：
- `Send` - 發送消息到指定代理
- `Broadcast` - 廣播消息到所有代理
- `Request` - 請求/回覆模式（同步）
- `Subscribe` - 訂閱主題

**消息類型**：
```go
TypeTaskAssign      // 任務分配
TypeTaskResult      // 任務結果
TypeTaskProgress    // 任務進度
TypeTaskCancel      // 任務取消
TypeHealthCheck     // 健康檢查
TypeHealthResponse  // 健康回覆
TypeAgentRegister   // 代理註冊
TypeConsensus       // 共識請求
```

**優先級**：
```go
PriorityLow      // 低優先級
PriorityNormal   // 普通
PriorityHigh     // 高優先級
PriorityCritical // 關鍵（立即處理）
```

### 2. State Machine（狀態機）

**狀態**：
```
Booting        -> 代理啟動中
Idle           -> 空閒等待
Processing     -> 處理任務中
WaitingForAgent -> 等待其他代理
Aggregating    -> 聚合結果
Done           -> 完成
Error          -> 錯誤
Shutdown       -> 已關閉
```

**事件**：
```
EventStart          // 啟動
EventTaskAssigned   // 任務分配
EventTaskCompleted  // 任務完成
EventTaskFailed     // 任務失敗
EventWaitForAgents // 等待代理
EventAllAgentsDone // 所有代理完成
EventAggregate     // 聚合
EventTimeout       // 超時
EventHeartbeat     // 心跳
EventShutdown      // 關閉
```

### 3. Guardian（守護者）

**功能**：

#### 心跳機制
- 每 5 秒發送心跳
- 15 秒無回覆視為超時
- 連續 3 次超時觸發處理

#### 任務超時
- 預設 5 分鐘超時
- 可按任務類型自定義
- 超時後自動重試或取消

#### 熔斷器
- 5 次失敗觸發熔斷
- 熔斷後一段時間嘗試恢復
- 防止連續失敗消耗資源

#### 死鎖檢測
- 30 秒無進步視為潛在死鎖
- 自動取消任務並重試
- 記錄死鎖歷史

### 4. SwarmExt（增強版 Swarm）

整合所有組件，提供：

```go
swarm := NewSwarmExt(10 * time.Minute)
swarm.Start()

// 註冊代理
swarm.RegisterAgent("agent1", RoleWorker, "Worker-1", 5)

// 提交任務
taskID := swarm.SubmitTask("實現用戶登錄功能", 1)

// 分配任務
swarm.AssignTask(taskID, "agent1")

// 查詢狀態
status := swarm.GetAgentStatus("agent1")
fmt.Printf("Agent %s 狀態: %s\n", status.Name, status.StateDesc)

// 發送代理間消息
swarm.SendToAgent("agent1", "agent2", messagebus.TypeTaskResult, payload)

// 獲取統計
stats := swarm.GetSwarmStats()

// 關閉
swarm.Shutdown()
```

## 使用範例

### 基本使用

```go
package main

import (
    "context"
    "time"
    
    "charm.land/crushcl/internal/agent"
    "charm.land/crushcl/internal/agent/messagebus"
)

func main() {
    // 創建 SwarmExt
    swarm := agent.NewSwarmExt(10 * time.Minute)
    swarm.Start()
    defer swarm.Shutdown()
    
    // 註冊代理
    swarm.RegisterAgent("coordinator", agent.RoleCoordinator, "Coordinator", 1)
    swarm.RegisterAgent("worker1", agent.RoleWorker, "Worker-1", 5)
    swarm.RegisterAgent("worker2", agent.RoleWorker, "Worker-2", 5)
    
    // 提交任務
    taskID := swarm.SubmitTask("實現用戶認證模組", 1)
    
    // 分配任務
    swarm.AssignTask(taskID, "worker1")
    
    // 等待一段時間
    time.Sleep(5 * time.Second)
    
    // 檢查狀態
    status := swarm.GetAgentStatus("worker1")
    println("Worker-1 State:", status.StateDesc)
    
    // 取消任務
    swarm.CancelTask(taskID)
}
```

### 代理間通信

```go
// 發送消息
err := swarm.SendToAgent("worker1", "worker2", messagebus.TypeTaskResult, map[string]interface{}{
    "task_id": "123",
    "result":  "完成",
})

// 請求/回覆
reply, err := swarm.RequestReply("worker1", "worker2", messagebus.TypeConsensus, payload, 5*time.Second)
if err != nil {
    println("請求超時:", err.Error())
}

// 廣播
err := swarm.Broadcast("coordinator", messagebus.TypeShutdown, nil)
```

### 自定義 Guardian 配置

```go
cfg := guardian.Config{
    HeartbeatInterval:     3 * time.Second,
    HeartbeatTimeout:      10 * time.Second,
    MissedHeartbeatsMax:   3,
    TaskTimeout:           10 * time.Minute,
    MaxRetries:            5,
    RetryDelay:            5 * time.Second,
    CircuitBreakerThreshold: 3,
    DeadlockDetection:      true,
    DeadlockTimeout:        60 * time.Second,
}

// 創建自定義 Guardian
mb := messagebus.NewInMemoryMessageBus()
g := guardian.NewGuardian(cfg, mb)
```

## 向後兼容

原有的 `swarm.go` 保持不變。`SwarmExt` 是增強版本，可以：

1. 單獨使用 `SwarmExt`
2. 與原有 `swarm` 混合使用
3. 逐步遷移到 `SwarmExt`

## 防止卡住的關鍵機制

### 1. 心跳檢測
```
每 5 秒 ──> 發送心跳 ──> 15 秒無回覆 ──> 標記為不健康
                  │
                  └──> 3 次超時 ──> 取消任務，重新分配
```

### 2. 任務追蹤
```
任務開始 ──> 記錄時間 ──> 定時檢查 ──> 超時 ──> 重試或熔斷
                │
                └──> 30 秒無進度 ──> 死鎖檢測
```

### 3. 熔斷器
```
失敗計數 ──> 達到閾值 ──> 打開熔斷 ──> 一段時間後 ──> 嘗試恢復
                              │
                              └──> 成功 ──> 重置計數
                              │
                              └──> 失敗 ──> 繼續熔斷
```

## 統計監控

```go
stats := swarm.GetSwarmStats()

// 輸出：
// {
//   "total_agents": 3,
//   "total_tasks": 10,
//   "pending_tasks": 2,
//   "running_tasks": 1,
//   "completed_tasks": 7,
//   "failed_tasks": 0,
//   "agents_by_role": {"coordinator": 1, "worker": 2},
//   "agent_status": {"coordinator": "idle", "worker1": "processing"},
//   "agent_states": {"idle": 2, "processing": 1},
//   "message_bus": {
//     "total_messages": 100,
//     "pending_messages": 5,
//     "subscribers": 3
//   }
// }
```
