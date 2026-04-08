# MessageBus 消息總線詳細設計

**版本**: v1.0  
**創建日期**: 2026-04-04  
**組件**: MessageBus  
**狀態**: ✅ 已完成

---

## 一、組件概述

### 1.1 功能說明

MessageBus 是 CrushCL 多代理系統的核心通信組件，負責代理間的訊息傳遞、訂閱機制和請求-回應模式。

### 1.2 核心能力

| 能力 | 說明 |
|------|------|
| **點對點發送** | 向指定代理發送訊息 |
| **廣播** | 向所有訂閱者廣播訊息 |
| **請求-回應** | 支援 RPC 風格的請求-回應模式 |
| **訂閱機制** | 基於主題的訂閱/發布模式 |

---

## 二、介面定義

### 2.1 Publisher 介面

```go
// Publisher 消息發布者介面
type Publisher interface {
    // Send 發送消息到指定代理
    Send(to AgentID, msg *Message) error
    
    // Broadcast 向所有訂閱者廣播消息
    Broadcast(msg *Message) error
    
    // Request 發送請求並等待回應 (帶超時)
    Request(to AgentID, msg *Message, timeout time.Duration) (*Message, error)
}
```

### 2.2 Subscriber 介面

```go
// Subscriber 消息訂閱者介面
type Subscriber interface {
    // Subscribe 訂閱主題
    Subscribe(topic string, handler MessageHandler) error
    
    // Unsubscribe 取消訂閱
    Unsubscribe(topic string) error
    
    // GetID 獲取訂閱者 ID
    GetID() AgentID
}
```

### 2.3 Message 結構

```go
// Message 消息結構
type Message struct {
    ID        MessageID       // 消息唯一標識
    Type      MessageType     // 消息類型
    From      AgentID        // 發送者 ID
    To        AgentID        // 接收者 ID (廣播時為空)
    Topic     string         // 主題 (用於訂閱)
    Payload   interface{}     // 消息內容
    Timestamp time.Time      // 發送時間
    ReplyTo   MessageID      // 回應的消息 ID (用於請求-回應)
}
```

### 2.4 MessageType 枚舉

```go
// MessageType 消息類型
type MessageType string

const (
    TypeTaskAssigned    MessageType = "task:assigned"      // 任務分配
    TypeTaskCompleted   MessageType = "task:completed"    // 任務完成
    TypeTaskFailed     MessageType = "task:failed"       // 任務失敗
    TypeHeartbeat      MessageType = "heartbeat"         // 心跳
    TypeHealthCheck    MessageType = "health:check"      // 健康檢查
    TypeStateChange    MessageType = "state:change"      // 狀態變化
    TypeShutdown       MessageType = "shutdown"          // 關閉信號
    TypeResult         MessageType = "result"            // 結果返回
)
```

---

## 三、實現細節

### 3.1 MessageBus 結構

```go
// MessageBus 消息總線實現
type MessageBus struct {
    mu         sync.RWMutex
    subscribers map[string]map[AgentID]MessageHandler // topic -> agent -> handler
    pending    map[MessageID]chan *Message             // 等待回應的消息
    agents     map[AgentID]*AgentInfo                  // 代理註冊表
    closed     atomic.Bool
}
```

### 3.2 核心方法

| 方法 | 說明 | 時間複雜度 |
|------|------|-----------|
| `New()` | 創建新的 MessageBus | O(1) |
| `Send(to AgentID, msg *Message)` | 發送點對點消息 | O(1) |
| `Broadcast(msg *Message)` | 廣播消息 | O(n) |
| `Request(to AgentID, msg *Message, timeout)` | 請求-回應模式 | O(1) |
| `Subscribe(topic string, handler)` | 訂閱主題 | O(1) |
| `Unsubscribe(topic string)` | 取消訂閱 | O(1) |
| `RegisterAgent(agent *AgentInfo)` | 註冊代理 | O(1) |
| `UnregisterAgent(id AgentID)` | 註銷代理 | O(1) |

### 3.3 執行流程

#### 發送消息流程

```
1. 調用 Send(to, msg)
2. 驗證接收者存在
3. 查找接收者的消息通道
4. 發送消息到通道
5. 返回成功/失敗
```

#### 訂閱流程

```
1. 調用 Subscribe(topic, handler)
2. 創建訂閱條目
3. 加入訂閱者列表
4. 返回成功
```

#### 請求-回應流程

```
1. 調用 Request(to, msg, timeout)
2. 生成回應通道
3. 發送消息 (帶 ReplyTo)
4. 等待回應或超時
5. 返回回應或錯誤
```

---

## 四、使用範例

### 4.1 基本發送

```go
bus := messagebus.New()

// 發送消息
err := bus.Send(agentID, &messagebus.Message{
    Type:    messagebus.TypeTaskAssigned,
    From:    coordinatorID,
    To:      agentID,
    Payload: task,
})
```

### 4.2 訂閱主題

```go
// 訂閱心跳主題
bus.Subscribe("heartbeat", func(msg *messagebus.Message) {
    fmt.Printf("收到心跳 from %s: %+v\n", msg.From, msg.Payload)
})
```

### 4.3 請求-回應

```go
// 請求健康檢查
response, err := bus.Request(agentID, &messagebus.Message{
    Type:    messagebus.TypeHealthCheck,
    Payload:  nil,
}, 5*time.Second)

if err != nil {
    fmt.Println("請求超時或失敗")
} else {
    fmt.Printf("收到回應: %+v\n", response.Payload)
}
```

### 4.4 廣播

```go
// 廣播關閉信號
bus.Broadcast(&messagebus.Message{
    Type:    messagebus.TypeShutdown,
    Payload: "系統關閉",
})
```

---

## 五、線程安全性

### 5.1 鎖策略

| 字段 | 鎖類型 | 說明 |
|------|--------|------|
| `subscribers` | `sync.RWMutex` | 讀寫鎖，支援並發讀取 |
| `pending` | `sync.RWMutex` | 讀寫鎖，用於請求-回應 |
| `agents` | `sync.RWMutex` | 讀寫鎖，代理註冊表 |
| `closed` | `atomic.Bool` | 原子操作，標記關閉狀態 |

### 5.2 並發約束

| 操作 | 鎖要求 |
|------|--------|
| 讀取 subscribers | RLock |
| 寫入 subscribers | Lock |
| 讀取 pending | RLock |
| 寫入 pending | Lock |
| 發送消息 | Lock (個別通道) |

---

## 六、錯誤處理

### 6.1 錯誤類型

```go
// ErrAgentNotFound 代理未找到
var ErrAgentNotFound = errors.New("agent not found")

// ErrTopicNotFound 主題未找到
var ErrTopicNotFound = errors.New("topic not found")

// ErrTimeout 請求超時
var ErrTimeout = errors.New("request timeout")

// ErrBusClosed 總線已關閉
var ErrBusClosed = errors.New("message bus closed")

// ErrInvalidMessage 無效消息
var ErrInvalidMessage = errors.New("invalid message")
```

### 6.2 處理策略

| 錯誤 | 處理方式 |
|------|----------|
| `ErrAgentNotFound` | 返回錯誤，調用者決定重試或取消 |
| `ErrTopicNotFound` | 靜默忽略，不阻塞發送者 |
| `ErrTimeout` | 關閉回應通道，返回超時錯誤 |
| `ErrBusClosed` | 返回錯誤，所有操作失敗 |

---

## 七、測試策略

### 7.1 單元測試

```go
func TestMessageBus_Send(t *testing.T) {
    bus := New()
    
    // 測試發送消息
    err := bus.Send("agent1", &Message{Type: TypeTaskAssigned})
    assert.NoError(t, err)
}

func TestMessageBus_Broadcast(t *testing.T) {
    bus := New()
    
    // 訂閱主題
    received := make([]*Message, 0)
    bus.Subscribe("test", func(msg *Message) {
        received = append(received, msg)
    })
    
    // 廣播
    bus.Broadcast(&Message{Type: TypeTaskAssigned, Topic: "test"})
    
    assert.Len(t, received, 1)
}
```

### 7.2 並發測試

```go
func TestMessageBus_Concurrent(t *testing.T) {
    bus := New()
    
    // 多個 goroutine 同時發送
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            bus.Send("agent1", &Message{Type: TypeHeartbeat})
        }(i)
    }
    
    wg.Wait()
    // 驗證沒有 panic 或死鎖
}
```

---

## 八、性能考量

### 8.1 緩衝區大小

| 場景 | 建議緩衝 |
|------|----------|
| 高頻心跳 | 100-1000 |
| 任務分配 | 10-100 |
| 結果返回 | 10-50 |

### 8.2 批量處理

```go
// 批量發送 (未來優化)
func (mb *MessageBus) SendBatch(messages []*Message) error
```

---

## 九、檔案位置

```
internal/agent/messagebus/
└── messagebus.go          # 主實現 (~400 行)
```

---

## 十、依賴關係

```
依賴:
    └─ 無 (完全獨立)

被依賴:
    ├─ SwarmExt (任務調度)
    ├─ Guardian (健康報告)
    ├─ HeartbeatAgent (心跳傳遞)
    └─ CircuitBreakerAgent (故障傳播)
```

---

## 十一、擴展方向

| 擴展項 | 說明 |
|--------|------|
| **持久化** | 消息持久化到磁碟，支援故障恢復 |
| **分片** | 多個 MessageBus 實例負載均衡 |
| **優先級** | 消息優先級隊列 |
| **追蹤** | 消息追蹤和鏈路追蹤 |

---

*文檔更新日期: 2026-04-04*
