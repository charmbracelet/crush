package statmachine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// State Types
// ============================================================================

// State 代理狀態
type State string

const (
	StateBooting         State = "booting"
	StateIdle            State = "idle"
	StateProcessing      State = "processing"
	StateWaitingForAgent State = "waiting_for_agent"
	StateAggregating     State = "aggregating"
	StateDone            State = "done"
	StateError           State = "error"
	StateShutdown        State = "shutdown"
)

// Event 觸發狀態轉換的事件
type Event string

const (
	EventStart         Event = "start"
	EventTaskAssigned  Event = "task_assigned"
	EventTaskCompleted Event = "task_completed"
	EventTaskFailed    Event = "task_failed"
	EventWaitForAgents Event = "wait_for_agents"
	EventAllAgentsDone Event = "all_agents_done"
	EventAggregate     Event = "aggregate"
	EventError         Event = "error"
	EventCancel        Event = "cancel"
	EventShutdown      Event = "shutdown"
	EventTimeout       Event = "timeout"
	EventHeartbeat     Event = "heartbeat"
	EventNoResponse    Event = "no_response"
)

// Transition 狀態轉換
type Transition struct {
	From  []State
	To    State
	Event Event
	Guard func(from State, event Event) bool
}

// ============================================================================
// Agent State Machine
// ============================================================================

// AgentStateMachine 代理狀態機
type AgentStateMachine struct {
	mu         sync.RWMutex
	agentID    string
	state      State
	prevState  State
	lastUpdate time.Time

	// 轉換歷史
	history     []StateTransition
	historySize int

	// 監聽器
	listeners   map[State][]func(State, State)
	muListeners sync.RWMutex

	// 超時配置
	timeouts map[State]time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// StateTransition 狀態轉換記錄
type StateTransition struct {
	From      State
	To        State
	Event     Event
	Timestamp time.Time
	Duration  time.Duration // 上一個狀態持續時間
}

// NewAgentStateMachine 創建新的狀態機
func NewAgentStateMachine(agentID string) *AgentStateMachine {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentStateMachine{
		agentID:     agentID,
		state:       StateBooting,
		lastUpdate:  time.Now(),
		listeners:   make(map[State][]func(State, State)),
		history:     make([]StateTransition, 0),
		historySize: 100,
		timeouts:    make(map[State]time.Duration),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetTimeouts 設置狀態超時
func (sm *AgentStateMachine) SetTimeouts(timeouts map[State]time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for state, duration := range timeouts {
		sm.timeouts[state] = duration
	}
}

// CurrentState 獲取當前狀態
func (sm *AgentStateMachine) CurrentState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// Transition 嘗試狀態轉換
func (sm *AgentStateMachine) Transition(event Event) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	from := sm.state
	to, ok := sm.getNextState(from, event)
	if !ok {
		return fmt.Errorf("invalid transition: %s from %s", event, from)
	}

	// 記錄轉換
	duration := time.Since(sm.lastUpdate)
	sm.history = append(sm.history, StateTransition{
		From:      from,
		To:        to,
		Event:     event,
		Timestamp: time.Now(),
		Duration:  duration,
	})

	// 限制歷史大小
	if len(sm.history) > sm.historySize {
		sm.history = sm.history[len(sm.history)-sm.historySize:]
	}

	sm.prevState = sm.state
	sm.state = to
	sm.lastUpdate = time.Now()

	// 通知監聽器
	sm.muListeners.RLock()
	defer sm.muListeners.RUnlock()
	for _, listener := range sm.listeners[from] {
		go listener(from, to)
	}
	for _, listener := range sm.listeners[State("*")] {
		go listener(from, to)
	}

	return nil
}

// getNextState 獲取下一個狀態
func (sm *AgentStateMachine) getNextState(from State, event Event) (State, bool) {
	transitions := getTransitions()

	for _, t := range transitions {
		for _, f := range t.From {
			if f == from && t.Event == event {
				if t.Guard != nil && !t.Guard(from, event) {
					continue
				}
				return t.To, true
			}
		}
	}
	return "", false
}

// CanTransition 檢查是否可以轉換
func (sm *AgentStateMachine) CanTransition(event Event) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.getNextState(sm.state, event)
	return ok
}

// GetHistory 獲取轉換歷史
func (sm *AgentStateMachine) GetHistory() []StateTransition {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]StateTransition, len(sm.history))
	copy(result, sm.history)
	return result
}

// GetLastDuration 獲取上一個狀態持續時間
func (sm *AgentStateMachine) GetLastDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if len(sm.history) == 0 {
		return 0
	}
	return sm.history[len(sm.history)-1].Duration
}

// IsTerminalState 檢查是否為終端狀態
func (sm *AgentStateMachine) IsTerminalState() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateDone || sm.state == StateError || sm.state == StateShutdown
}

// AddListener 添加狀態監聽器
func (sm *AgentStateMachine) AddListener(state State, listener func(from, to State)) {
	sm.muListeners.Lock()
	defer sm.muListeners.Unlock()
	sm.listeners[state] = append(sm.listeners[state], listener)
}

// RemoveListeners 移除所有監聽器
func (sm *AgentStateMachine) RemoveListeners() {
	sm.muListeners.Lock()
	defer sm.muListeners.Unlock()
	sm.listeners = make(map[State][]func(State, State))
}

// Shutdown 關閉狀態機
func (sm *AgentStateMachine) Shutdown() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.state != StateShutdown {
		sm.history = append(sm.history, StateTransition{
			From:      sm.state,
			To:        StateShutdown,
			Event:     EventShutdown,
			Timestamp: time.Now(),
		})
		sm.state = StateShutdown
	}
	sm.cancel()
}

// ============================================================================
// State Transitions Definition
// ============================================================================

func getTransitions() []Transition {
	return []Transition{
		// 啟動流程
		{[]State{StateBooting}, StateIdle, EventStart, nil},

		// 正常任務流程
		{[]State{StateIdle}, StateProcessing, EventTaskAssigned, nil},
		{[]State{StateProcessing}, StateIdle, EventTaskCompleted, nil},
		{[]State{StateProcessing}, StateError, EventTaskFailed, nil},

		// 等待其他代理
		{[]State{StateProcessing}, StateWaitingForAgent, EventWaitForAgents, nil},
		{[]State{StateWaitingForAgent}, StateAggregating, EventAllAgentsDone, nil},
		{[]State{StateAggregating}, StateDone, EventAggregate, nil},

		// 錯誤處理
		{[]State{StateProcessing, StateWaitingForAgent}, StateError, EventError, nil},
		{[]State{StateError}, StateIdle, EventStart, nil},

		// 超時處理
		{[]State{StateWaitingForAgent}, StateError, EventTimeout, nil},
		{[]State{StateProcessing}, StateError, EventNoResponse, nil},

		// 取消
		{[]State{StateIdle, StateProcessing, StateWaitingForAgent}, StateIdle, EventCancel, nil},

		// 心跳保持活躍
		{[]State{StateProcessing}, StateProcessing, EventHeartbeat, nil},

		// 關閉
		{[]State{StateIdle, StateProcessing, StateWaitingForAgent, StateError}, StateShutdown, EventShutdown, nil},
		{[]State{StateDone}, StateShutdown, EventShutdown, nil},
	}
}

// ============================================================================
// State Description
// ============================================================================

// StateDescription 狀態描述
var StateDescription = map[State]string{
	StateBooting:         "代理正在啟動",
	StateIdle:            "代理空閒，等待任務",
	StateProcessing:      "代理正在處理任務",
	StateWaitingForAgent: "等待其他代理完成",
	StateAggregating:     "正在聚合結果",
	StateDone:            "任務已完成",
	StateError:           "發生錯誤",
	StateShutdown:        "已關閉",
}

// Description 獲取狀態描述
func (s State) Description() string {
	if desc, ok := StateDescription[s]; ok {
		return desc
	}
	return "未知狀態"
}
