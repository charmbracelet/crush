package messagebus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Message Types
// ============================================================================

// MessagePriority 消息優先級
type MessagePriority int

const (
	PriorityLow MessagePriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// MessageType 消息類型
type MessageType string

const (
	TypeTaskAssign      MessageType = "task:assign"
	TypeTaskResult      MessageType = "task:result"
	TypeTaskProgress    MessageType = "task:progress"
	TypeTaskCancel      MessageType = "task:cancel"
	TypeTaskFailed      MessageType = "task:failed"
	TypeHealthCheck     MessageType = "health:check"
	TypeHealthResponse  MessageType = "health:response"
	TypeAgentRegister   MessageType = "agent:register"
	TypeAgentUnregister MessageType = "agent:unregister"
	TypeAgentSync       MessageType = "agent:sync"
	TypeConsensus       MessageType = "consensus:request"
	TypeConsensusResult MessageType = "consensus:result"
	TypeCheckpoint      MessageType = "checkpoint:save"
	TypeRollback        MessageType = "rollback:request"
	TypeShutdown        MessageType = "system:shutdown"
)

// Message 消息結構
type Message struct {
	ID        string          `json:"id"`
	From      string          `json:"from"`
	To        string          `json:"to"` // "" = broadcast
	Type      MessageType     `json:"type"`
	Payload   interface{}     `json:"payload"`
	Priority  MessagePriority `json:"priority"`
	Timestamp time.Time       `json:"timestamp"`
	ReplyTo   string          `json:"reply_to"`   // 回覆的消息ID
	ExpiresAt *time.Time      `json:"expires_at"` // 過期時間
	Retry     int             `json:"retry"`      // 重試次數
	MaxRetry  int             `json:"max_retry"`  // 最大重試次數
}

// NewMessage 創建新消息
func NewMessage(from, to string, msgType MessageType, payload interface{}) *Message {
	return &Message{
		ID:        generateMessageID(),
		From:      from,
		To:        to,
		Type:      msgType,
		Payload:   payload,
		Priority:  PriorityNormal,
		Timestamp: time.Now(),
		MaxRetry:  3,
	}
}

// NewPriorityMessage 創建帶優先級的消息
func NewPriorityMessage(from, to string, msgType MessageType, payload interface{}, priority MessagePriority) *Message {
	msg := NewMessage(from, to, msgType, payload)
	msg.Priority = priority
	return msg
}

// IsExpired 檢查消息是否過期
func (m *Message) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// ShouldRetry 是否應該重試
func (m *Message) ShouldRetry() bool {
	return m.Retry < m.MaxRetry
}

// ============================================================================
// Subscriber 訂閱者
// ============================================================================

// Subscriber 消息訂閱者接口
type Subscriber interface {
	Receive(msg *Message)
	ID() string
}

// Subscription 訂閱配置
type Subscription struct {
	AgentID    string
	Topics     []MessageType
	Handler    func(msg *Message)
	BufferSize int
}

// ============================================================================
// MessageBus 消息總線接口
// ============================================================================

// MessageBus 消息總線接口
type MessageBus interface {
	// 發送消息
	Send(ctx context.Context, msg *Message) error
	Broadcast(ctx context.Context, msg *Message) error
	Request(ctx context.Context, msg *Message, timeout time.Duration) (*Message, error)
	Reply(ctx context.Context, original *Message, payload interface{}) error

	// 訂閱管理
	Subscribe(sub *Subscription) func()
	Unsubscribe(agentID string)

	// 查詢
	GetInbox(agentID string) []*Message
	GetStats() MessageBusStats

	// 生命週期
	Shutdown()
}

// MessageBusStats 消息總線統計
type MessageBusStats struct {
	TotalMessages   int64            `json:"total_messages"`
	PendingMessages int64            `json:"pending_messages"`
	Subscribers     int              `json:"subscribers"`
	QueuesByAgent   map[string]int64 `json:"queues_by_agent"`
}

// ============================================================================
// InMemoryMessageBus 內存消息總線實現
// ============================================================================

// inbox 代理收件箱
type inbox struct {
	mu       sync.Mutex
	messages []*Message
	notify   chan struct{}
}

// InMemoryMessageBus 內存消息總線
type InMemoryMessageBus struct {
	mu           sync.RWMutex
	subscribers  map[string]*Subscription // agentID -> subscription
	inboxes      map[string]*inbox        // agentID -> inbox
	broadcast    *inbox                   // broadcast messages
	topics       map[MessageType][]string // topic -> agentIDs
	stats        MessageBusStats
	pendingQueue map[string][]*Message // priority queues
	muStats      sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewInMemoryMessageBus 創建新的內存消息總線
func NewInMemoryMessageBus() *InMemoryMessageBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &InMemoryMessageBus{
		subscribers:  make(map[string]*Subscription),
		inboxes:      make(map[string]*inbox),
		broadcast:    &inbox{messages: make([]*Message, 0), notify: make(chan struct{}, 100)},
		topics:       make(map[MessageType][]string),
		pendingQueue: make(map[string][]*Message),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// GetInbox 獲取代理的收件箱
func (mb *InMemoryMessageBus) GetInbox(agentID string) []*Message {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	ib, ok := mb.inboxes[agentID]
	if !ok {
		return nil
	}

	ib.mu.Lock()
	defer ib.mu.Unlock()

	result := make([]*Message, len(ib.messages))
	copy(result, ib.messages)
	return result
}

// Subscribe 訂閱主題
func (mb *InMemoryMessageBus) Subscribe(sub *Subscription) func() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.subscribers[sub.AgentID] = sub

	// 創建個人 inbox
	if _, ok := mb.inboxes[sub.AgentID]; !ok {
		mb.inboxes[sub.AgentID] = &inbox{
			messages: make([]*Message, 0),
			notify:   make(chan struct{}, sub.BufferSize),
		}
	}

	// 註冊主題
	for _, topic := range sub.Topics {
		mb.topics[topic] = append(mb.topics[topic], sub.AgentID)
	}

	return func() {
		mb.Unsubscribe(sub.AgentID)
	}
}

// Unsubscribe 取消訂閱
func (mb *InMemoryMessageBus) Unsubscribe(agentID string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	sub, ok := mb.subscribers[agentID]
	if !ok {
		return
	}

	// 從主題中移除
	for _, topic := range sub.Topics {
		agents := mb.topics[topic]
		for i, id := range agents {
			if id == agentID {
				mb.topics[topic] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
	}

	delete(mb.subscribers, agentID)
}

// Send 發送消息到指定代理
func (mb *InMemoryMessageBus) Send(ctx context.Context, msg *Message) error {
	mb.muStats.Lock()
	mb.stats.TotalMessages++
	mb.stats.PendingMessages++
	mb.muStats.Unlock()

	// 驗證目標
	if msg.To == "" {
		return fmt.Errorf("message destination is required for Send")
	}

	mb.mu.RLock()
	ib, ok := mb.inboxes[msg.To]
	mb.mu.RUnlock()

	if !ok {
		// 自動創建 inbox
		mb.mu.Lock()
		ib = &inbox{
			messages: make([]*Message, 0),
			notify:   make(chan struct{}, 100),
		}
		mb.inboxes[msg.To] = ib
		mb.mu.Unlock()
	}

	ib.mu.Lock()
	ib.messages = append(ib.messages, msg)
	ib.mu.Unlock()

	// 通知新消息
	select {
	case ib.notify <- struct{}{}:
	default:
	}

	return nil
}

// Broadcast 廣播消息
func (mb *InMemoryMessageBus) Broadcast(ctx context.Context, msg *Message) error {
	mb.muStats.Lock()
	mb.stats.TotalMessages++
	mb.muStats.Unlock()

	msg.To = "" // broadcast marker

	mb.mu.RLock()
	broadcast := mb.broadcast
	mb.mu.RUnlock()

	broadcast.mu.Lock()
	broadcast.messages = append(broadcast.messages, msg)
	broadcast.mu.Unlock()

	// 通知所有訂閱者
	broadcast.notify <- struct{}{}

	// 通知相關主題訂閱者
	mb.mu.RLock()
	for _, topic := range mb.topics[msg.Type] {
		if ib, ok := mb.inboxes[topic]; ok {
			select {
			case ib.notify <- struct{}{}:
			default:
			}
		}
	}
	mb.mu.RUnlock()

	return nil
}

// Request 發送請求並等待回覆
func (mb *InMemoryMessageBus) Request(ctx context.Context, msg *Message, timeout time.Duration) (*Message, error) {
	replyCh := make(chan *Message, 1)
	replyTo := msg.ID

	// 設置回覆監聽
	mb.mu.Lock()
	ib, ok := mb.inboxes[msg.To]
	if !ok {
		ib = &inbox{
			messages: make([]*Message, 0),
			notify:   make(chan struct{}, 100),
		}
		mb.inboxes[msg.To] = ib
	}
	mb.mu.Unlock()

	// 在目標 inbox 添加回覆處理
	done := make(chan struct{})
	defer close(done)

	go func() {
		ib.mu.Lock()
		for {
			select {
			case <-done:
				ib.mu.Unlock()
				return
			default:
				// 檢查是否有回覆
				for i, m := range ib.messages {
					if m.ReplyTo == replyTo && m.Type == TypeHealthResponse || m.Type == TypeTaskResult || m.Type == TypeConsensusResult {
						replyCh <- m
						// 移除已處理的回覆
						ib.messages = append(ib.messages[:i], ib.messages[i+1:]...)
						ib.mu.Unlock()
						return
					}
				}
				ib.mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				ib.mu.Lock()
			}
		}
	}()

	// 發送消息
	if err := mb.Send(ctx, msg); err != nil {
		return nil, err
	}

	// 等待回覆或超時
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case reply := <-replyCh:
		return reply, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timeout after %v", timeout)
	}
}

// Reply 發送回覆
func (mb *InMemoryMessageBus) Reply(ctx context.Context, original *Message, payload interface{}) error {
	reply := &Message{
		ID:        generateMessageID(),
		From:      original.To,
		To:        original.From,
		Type:      original.Type,
		Payload:   payload,
		Priority:  PriorityNormal,
		Timestamp: time.Now(),
		ReplyTo:   original.ID,
	}

	return mb.Send(ctx, reply)
}

// GetStats 獲取統計
func (mb *InMemoryMessageBus) GetStats() MessageBusStats {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	mb.muStats.Lock()
	stats := mb.stats
	stats.Subscribers = len(mb.subscribers)
	stats.QueuesByAgent = make(map[string]int64)
	for id, ib := range mb.inboxes {
		ib.mu.Lock()
		stats.QueuesByAgent[id] = int64(len(ib.messages))
		ib.mu.Unlock()
	}
	mb.muStats.Unlock()

	return stats
}

// Shutdown 關閉消息總線
func (mb *InMemoryMessageBus) Shutdown() {
	mb.cancel()
}

// ============================================================================
// Helper Functions
// ============================================================================

var messageIDCounter int64

func generateMessageID() string {
	messageIDCounter++
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), messageIDCounter)
}
