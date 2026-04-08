package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crushcl/internal/agent/guardian"
	"github.com/charmbracelet/crushcl/internal/agent/messagebus"
	"github.com/charmbracelet/crushcl/internal/agent/statmachine"
)

// ============================================================================
// SwarmExt 增強版 Swarm - 整合溝通框架和防卡住機制
// ============================================================================

// SwarmExt 增強版 swarm
type SwarmExt struct {
	mu         sync.RWMutex
	agents     map[string]*AgentInfo
	tasks      map[string]*Task
	messageBus *messagebus.InMemoryMessageBus
	guardian   *guardian.Guardian
	states     map[string]*statmachine.AgentStateMachine

	// 配置
	timeout    time.Duration
	maxRetries int

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSwarmExt 創建增強版 swarm
func NewSwarmExt(timeout time.Duration) *SwarmExt {
	ctx, cancel := context.WithCancel(context.Background())
	mb := messagebus.NewInMemoryMessageBus()
	cfg := guardian.DefaultConfig()

	return &SwarmExt{
		agents:     make(map[string]*AgentInfo),
		tasks:      make(map[string]*Task),
		messageBus: mb,
		guardian:   guardian.NewGuardian(cfg, mb),
		states:     make(map[string]*statmachine.AgentStateMachine),
		timeout:    timeout,
		maxRetries: 3,
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
}

// ============================================================================
// Agent Management
// ============================================================================

// RegisterAgent 註冊代理（增強版）
func (s *SwarmExt) RegisterAgent(id string, role AgentRole, name string, capacity int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[id] = &AgentInfo{
		ID:       id,
		Role:     role,
		Name:     name,
		Busy:     false,
		Tasks:    []string{},
		Capacity: capacity,
	}

	// 初始化狀態機
	s.states[id] = statmachine.NewAgentStateMachine(id)
	s.states[id].SetTimeouts(map[statmachine.State]time.Duration{
		statmachine.StateWaitingForAgent: 30 * time.Second,
		statmachine.StateProcessing:      5 * time.Minute,
	})

	// 註冊到 Guardian
	s.guardian.RegisterAgent(id)

	// 訂閱消息
	s.messageBus.Subscribe(&messagebus.Subscription{
		AgentID:    id,
		Topics:     []messagebus.MessageType{},
		Handler:    func(msg *messagebus.Message) { s.handleMessage(id, msg) },
		BufferSize: 100,
	})

	// 啟動心跳
	s.guardian.StartHeartbeat(id)

	// 觸發狀態轉換
	s.states[id].Transition(statmachine.EventStart)
}

// UnregisterAgent 取消註冊代理
func (s *SwarmExt) UnregisterAgent(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.guardian.StopHeartbeat(id)
	s.guardian.UnregisterAgent(id)
	s.messageBus.Unsubscribe(id)
	delete(s.agents, id)
	delete(s.states, id)
}

// ============================================================================
// Message Handling
// ============================================================================

// handleMessage 處理消息
func (s *SwarmExt) handleMessage(agentID string, msg *messagebus.Message) {
	// 根據消息類型處理
	switch msg.Type {
	case messagebus.TypeTaskAssign:
		s.handleTaskAssign(agentID, msg)
	case messagebus.TypeTaskProgress:
		s.handleTaskProgress(agentID, msg)
	case messagebus.TypeTaskResult:
		s.handleTaskResult(agentID, msg)
	case messagebus.TypeTaskCancel:
		s.handleTaskCancel(agentID, msg)
	case messagebus.TypeHealthCheck:
		s.handleHealthCheck(agentID, msg)
	case messagebus.TypeHealthResponse:
		s.handleHealthResponse(agentID, msg)
	}
}

// handleTaskAssign 處理任務分配
func (s *SwarmExt) handleTaskAssign(agentID string, msg *messagebus.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新狀態
	if state, ok := s.states[agentID]; ok {
		state.Transition(statmachine.EventTaskAssigned)
	}

	// 更新任務
	taskID := msg.Payload.(map[string]interface{})["task_id"].(string)
	if task, ok := s.tasks[taskID]; ok {
		task.AssignedTo = agentID
		task.Status = TaskRunning
	}

	// 更新代理
	if agent, ok := s.agents[agentID]; ok {
		agent.Busy = true
		agent.Tasks = append(agent.Tasks, taskID)
	}

	// 追蹤任務
	s.guardian.TrackTask(taskID, agentID, 0)
}

// handleTaskProgress 處理任務進度
func (s *SwarmExt) handleTaskProgress(agentID string, msg *messagebus.Message) {
	payload := msg.Payload.(map[string]interface{})
	taskID := payload["task_id"].(string)
	progress := int(payload["progress"].(float64))

	s.guardian.UpdateProgress(taskID, progress)
}

// handleTaskResult 處理任務結果
func (s *SwarmExt) handleTaskResult(agentID string, msg *messagebus.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload := msg.Payload.(map[string]interface{})
	taskID := payload["task_id"].(string)

	task, ok := s.tasks[taskID]
	if !ok {
		return
	}

	// 更新狀態
	if state, ok := s.states[agentID]; ok {
		state.Transition(statmachine.EventTaskCompleted)
	}

	// 標記任務完成
	task.Status = TaskCompleted
	now := time.Now()
	task.CompletedAt = &now

	// 釋放代理
	if agent, ok := s.agents[agentID]; ok {
		agent.Busy = false
		agent.Tasks = s.removeTaskFromAgent(agent, taskID)
	}

	// 完成追蹤
	s.guardian.CompleteTask(taskID)
	s.guardian.RecordSuccess(agentID)
}

// handleTaskCancel 處理任務取消
func (s *SwarmExt) handleTaskCancel(agentID string, msg *messagebus.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload := msg.Payload.(map[string]interface{})
	taskID := payload["task_id"].(string)
	reason := payload["reason"].(string)

	task, ok := s.tasks[taskID]
	if !ok {
		return
	}

	// 標記任務失敗
	task.Status = TaskCancelled

	// 釋放代理
	if agent, ok := s.agents[agentID]; ok {
		agent.Busy = false
		agent.Tasks = s.removeTaskFromAgent(agent, taskID)
	}

	// 取消追蹤
	s.guardian.CancelTask(taskID)

	// 記錄失敗
	s.guardian.RecordFailure(agentID, fmt.Errorf("task cancelled: %s", reason))
}

// handleHealthCheck 處理健康檢查
func (s *SwarmExt) handleHealthCheck(agentID string, msg *messagebus.Message) {
	reply := messagebus.NewMessage("guardian", agentID, messagebus.TypeHealthResponse, map[string]interface{}{
		"status": "ok",
	})
	reply.ReplyTo = msg.ID
	s.messageBus.Send(s.ctx, reply)
}

// handleHealthResponse 處理健康回覆
func (s *SwarmExt) handleHealthResponse(agentID string, msg *messagebus.Message) {
	s.guardian.ReceiveHeartbeat(agentID)
}

// ============================================================================
// Task Management
// ============================================================================

// SubmitTask 提交任務（增強版）
func (s *SwarmExt) SubmitTask(description string, priority int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := generateTaskID()
	s.tasks[taskID] = &Task{
		ID:          taskID,
		Description: description,
		Status:      TaskPending,
		Priority:    priority,
		CreatedAt:   time.Now(),
	}

	return taskID
}

// AssignTask 分配任務（增強版）
func (s *SwarmExt) AssignTask(taskID, agentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok || task.Status != TaskPending {
		return false
	}

	agent, ok := s.agents[agentID]
	if !ok {
		return false
	}

	if agent.Busy && len(agent.Tasks) >= agent.Capacity {
		return false
	}

	// 分配任務
	task.AssignedTo = agentID
	task.Status = TaskRunning
	agent.Busy = true
	agent.Tasks = append(agent.Tasks, taskID)

	// 發送消息
	s.messageBus.Send(s.ctx, messagebus.NewPriorityMessage(
		"swarm",
		agentID,
		messagebus.TypeTaskAssign,
		map[string]interface{}{
			"task_id":     taskID,
			"description": task.Description,
		},
		messagebus.PriorityHigh,
	))

	// 更新狀態
	if state, ok := s.states[agentID]; ok {
		state.Transition(statmachine.EventTaskAssigned)
	}

	// 追蹤任務
	s.guardian.TrackTask(taskID, agentID, 0)

	return true
}

// CompleteTask 完成任務（增強版）
func (s *SwarmExt) CompleteTask(taskID string, result *fantasy.AgentResult, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return
	}

	if err != nil {
		task.Status = TaskFailed
		task.Error = err
	} else {
		task.Status = TaskCompleted
		task.Result = result
		now := time.Now()
		task.CompletedAt = &now
	}

	// 釋放代理
	if task.AssignedTo != "" {
		if agent, ok := s.agents[task.AssignedTo]; ok {
			agent.Busy = false
			agent.Tasks = s.removeTaskFromAgent(agent, taskID)
		}

		// 更新狀態
		if state, ok := s.states[task.AssignedTo]; ok {
			if err != nil {
				state.Transition(statmachine.EventTaskFailed)
			} else {
				state.Transition(statmachine.EventTaskCompleted)
			}
		}
	}

	// 完成追蹤
	if err != nil {
		s.guardian.RecordFailure(task.AssignedTo, err)
	} else {
		s.guardian.CompleteTask(taskID)
		s.guardian.RecordSuccess(task.AssignedTo)
	}

	// 發送結果消息
	s.messageBus.Send(s.ctx, messagebus.NewMessage(
		task.AssignedTo,
		"swarm",
		messagebus.TypeTaskResult,
		map[string]interface{}{
			"task_id": taskID,
			"success": err == nil,
		},
	))
}

// CancelTask 取消任務（增強版）
func (s *SwarmExt) CancelTask(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok || task.Status == TaskCompleted || task.Status == TaskCancelled {
		return false
	}

	task.Status = TaskCancelled

	// 釋放代理
	if task.AssignedTo != "" {
		if agent, ok := s.agents[task.AssignedTo]; ok {
			agent.Busy = false
			agent.Tasks = s.removeTaskFromAgent(agent, taskID)
		}
	}

	// 發送取消消息
	s.messageBus.Send(s.ctx, messagebus.NewPriorityMessage(
		"swarm",
		task.AssignedTo,
		messagebus.TypeTaskCancel,
		map[string]interface{}{
			"task_id": taskID,
			"reason":  "user_cancelled",
		},
		messagebus.PriorityHigh,
	))

	// 取消追蹤
	s.guardian.CancelTask(taskID)

	return true
}

// ============================================================================
// Query Methods
// ============================================================================

// GetTaskStatus 獲取任務狀態
func (s *SwarmExt) GetTaskStatus(taskID string) TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if task, ok := s.tasks[taskID]; ok {
		return task.Status
	}
	return TaskCancelled
}

// GetAgentStatus 獲取代理狀態（增強版）
func (s *SwarmExt) GetAgentStatus(agentID string) *AgentStatusExt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if agent, ok := s.agents[agentID]; ok {
		status := s.guardian.GetAgentStatus(agentID)
		state := s.states[agentID].CurrentState()

		return &AgentStatusExt{
			AgentInfo:     agent,
			State:         state,
			StateDesc:     state.Description(),
			LastHeartbeat: status.LastHeartbeat,
			MissedBeats:   status.MissedBeats,
			IsBlocked:     status.IsBlocked,
			IsCircuitOpen: status.IsCircuitOpen,
			ErrorMessage:  status.ErrorMessage,
		}
	}
	return nil
}

// AgentStatusExt 增強版代理狀態
type AgentStatusExt struct {
	*AgentInfo
	State         statmachine.State
	StateDesc     string
	LastHeartbeat time.Time
	MissedBeats   int
	IsBlocked     bool
	IsCircuitOpen bool
	ErrorMessage  string
}

// GetAvailableAgents 獲取可用代理
func (s *SwarmExt) GetAvailableAgents(role AgentRole) []*AgentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var available []*AgentInfo
	for _, agent := range s.agents {
		if agent.Role == role && !agent.Busy && agent.Capacity > len(agent.Tasks) {
			// 檢查健康狀態
			if s.guardian.IsAgentHealthy(agent.ID) {
				available = append(available, agent)
			}
		}
	}
	return available
}

// GetAllAgentStatus 獲取所有代理狀態
func (s *SwarmExt) GetAllAgentStatus() map[string]*AgentStatusExt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*AgentStatusExt)
	for id := range s.agents {
		result[id] = s.GetAgentStatus(id)
	}
	return result
}

// GetSwarmStats 獲取 Swarm 統計（增強版）
func (s *SwarmExt) GetSwarmStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total_agents":    len(s.agents),
		"total_tasks":     len(s.tasks),
		"pending_tasks":   0,
		"running_tasks":   0,
		"completed_tasks": 0,
		"failed_tasks":    0,
		"agents_by_role":  map[AgentRole]int{},
		"agent_status":    map[string]string{},
		"message_bus":     s.messageBus.GetStats(),
	}

	// 狀態統計
	states := map[statmachine.State]int{}
	for id, agent := range s.agents {
		stats["agents_by_role"].(map[AgentRole]int)[agent.Role]++
		state := s.states[id].CurrentState()
		stats["agent_status"].(map[string]string)[id] = string(state)
		states[state]++
	}
	stats["agent_states"] = states

	// 任務統計
	for _, task := range s.tasks {
		switch task.Status {
		case TaskPending:
			stats["pending_tasks"] = stats["pending_tasks"].(int) + 1
		case TaskRunning:
			stats["running_tasks"] = stats["running_tasks"].(int) + 1
		case TaskCompleted:
			stats["completed_tasks"] = stats["completed_tasks"].(int) + 1
		case TaskFailed:
			stats["failed_tasks"] = stats["failed_tasks"].(int) + 1
		}
	}

	return stats
}

// ============================================================================
// Inter-Agent Communication
// ============================================================================

// SendToAgent 發送消息給指定代理
func (s *SwarmExt) SendToAgent(from, to string, msgType messagebus.MessageType, payload interface{}) error {
	msg := messagebus.NewMessage(from, to, msgType, payload)
	return s.messageBus.Send(s.ctx, msg)
}

// Broadcast 廣播消息
func (s *SwarmExt) Broadcast(from string, msgType messagebus.MessageType, payload interface{}) error {
	msg := messagebus.NewMessage(from, "", msgType, payload)
	return s.messageBus.Broadcast(s.ctx, msg)
}

// RequestReply 發送請求並等待回覆
func (s *SwarmExt) RequestReply(from, to string, msgType messagebus.MessageType, payload interface{}, timeout time.Duration) (*messagebus.Message, error) {
	msg := messagebus.NewMessage(from, to, msgType, payload)
	return s.messageBus.Request(s.ctx, msg, timeout)
}

// ============================================================================
// Lifecycle
// ============================================================================

// Start 啟動 SwarmExt
func (s *SwarmExt) Start() {
	s.guardian.Start()
}

// Shutdown 關閉 SwarmExt
func (s *SwarmExt) Shutdown() {
	s.cancel()

	// 關閉所有代理
	for id := range s.agents {
		s.guardian.StopHeartbeat(id)
		s.guardian.UnregisterAgent(id)
	}

	s.guardian.Shutdown()
	s.messageBus.Shutdown()

	close(s.done)
}

// ============================================================================
// Helper
// ============================================================================

func (s *SwarmExt) removeTaskFromAgent(agent *AgentInfo, taskID string) []string {
	for i, t := range agent.Tasks {
		if t == taskID {
			return append(agent.Tasks[:i], agent.Tasks[i+1:]...)
		}
	}
	return agent.Tasks
}
