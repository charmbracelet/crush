package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"charm.land/fantasy"
)

type AgentRole string

const (
	RoleCoordinator AgentRole = "coordinator"
	RoleWorker     AgentRole = "worker"
	RoleReviewer   AgentRole = "reviewer"
	RolePlanner    AgentRole = "planner"
)

type AgentInfo struct {
	ID       string
	Role     AgentRole
	Name     string
	Busy     bool
	Tasks    []string
	Capacity int
}

type Task struct {
	ID          string
	Description string
	AssignedTo  string
	Status      TaskStatus
	Priority    int
	CreatedAt   time.Time
	CompletedAt *time.Time
	Result      *fantasy.AgentResult
	Error       error
	SubTasks    []string
}

type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskCompleted
	TaskFailed
	TaskCancelled
)

type Message struct {
	From      string
	To        string
	Type      MessageType
	Content   string
	Timestamp time.Time
	TaskID    string
}

type MessageType int

const (
	MsgTaskAssign MessageType = iota
	MsgTaskResult
	MsgTaskProgress
	MsgTaskCancel
	MsgHealthCheck
	MsgHealthResponse
)

type swarm struct {
	mu         sync.RWMutex
	agents     map[string]*AgentInfo
	tasks      map[string]*Task
	messages   chan Message
	taskQueue  chan string
	results    *ResultAggregator
	ctx        context.Context
	cancel     context.CancelFunc
	timeout    time.Duration
	maxRetries int
}

type ResultAggregator struct {
	mu      sync.Mutex
	results map[string]interface{}
}

func newResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		results: make(map[string]interface{}),
	}
}

func (ra *ResultAggregator) AddResult(taskID string, result interface{}) {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	ra.results[taskID] = result
}

func (ra *ResultAggregator) GetResult(taskID string) (interface{}, bool) {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	result, ok := ra.results[taskID]
	return result, ok
}

func newSwarm(timeout time.Duration) *swarm {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &swarm{
		agents:     make(map[string]*AgentInfo),
		tasks:      make(map[string]*Task),
		messages:   make(chan Message, 100),
		taskQueue:  make(chan string, 50),
		results:    newResultAggregator(),
		ctx:        ctx,
		cancel:     cancel,
		timeout:    timeout,
		maxRetries: 3,
	}
}

func (s *swarm) StartDispatch() {
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case taskID := <-s.taskQueue:
				s.dispatchTask(taskID)
			}
		}
	}()
}

func (s *swarm) dispatchTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok || task.Status != TaskPending {
		return
	}

	for _, agent := range s.agents {
		if agent.Role == RoleWorker && !agent.Busy && len(agent.Tasks) < agent.Capacity {
			task.AssignedTo = agent.ID
			task.Status = TaskRunning
			agent.Busy = true
			agent.Tasks = append(agent.Tasks, taskID)

			select {
			case s.messages <- Message{
				From:      "swarm",
				To:        agent.ID,
				Type:      MsgTaskAssign,
				Content:   task.Description,
				Timestamp: time.Now(),
				TaskID:    taskID,
			}:
			default:
			}
			return
		}
	}
}

func (s *swarm) RegisterAgent(id string, role AgentRole, name string, capacity int) {
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
}

func (s *swarm) SubmitTask(description string, priority int) string {
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

	select {
	case s.taskQueue <- taskID:
	default:
	}

	return taskID
}

func (s *swarm) AssignTask(taskID, agentID string) bool {
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

	task.AssignedTo = agentID
	task.Status = TaskRunning
	agent.Busy = true
	agent.Tasks = append(agent.Tasks, taskID)

	s.messages <- Message{
		From:      "swarm",
		To:        agentID,
		Type:      MsgTaskAssign,
		Content:   task.Description,
		Timestamp: time.Now(),
		TaskID:    taskID,
	}

	return true
}

func (s *swarm) CompleteTask(taskID string, result *fantasy.AgentResult, err error) {
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

	if task.AssignedTo != "" {
		if agent, ok := s.agents[task.AssignedTo]; ok {
			agent.Busy = false
			agent.Tasks = s.removeTaskFromAgent(agent, taskID)
		}
	}

	s.results.AddResult(taskID, result)

	s.messages <- Message{
		From:      task.AssignedTo,
		To:        "swarm",
		Type:      MsgTaskResult,
		Content:   "",
		Timestamp: time.Now(),
		TaskID:    taskID,
	}
}

func (s *swarm) GetTaskStatus(taskID string) TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if task, ok := s.tasks[taskID]; ok {
		return task.Status
	}
	return TaskCancelled
}

func (s *swarm) GetAgentStatus(agentID string) *AgentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if agent, ok := s.agents[agentID]; ok {
		return agent
	}
	return nil
}

func (s *swarm) GetAvailableAgents(role AgentRole) []*AgentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var available []*AgentInfo
	for _, agent := range s.agents {
		if agent.Role == role && !agent.Busy && agent.Capacity > len(agent.Tasks) {
			available = append(available, agent)
		}
	}
	return available
}

func (s *swarm) GetSwarmStats() map[string]interface{} {
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
	}

	for _, agent := range s.agents {
		stats["agents_by_role"].(map[AgentRole]int)[agent.Role]++
	}

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

func (s *swarm) CancelTask(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok || task.Status == TaskCompleted || task.Status == TaskCancelled {
		return false
	}

	task.Status = TaskCancelled

	if task.AssignedTo != "" {
		if agent, ok := s.agents[task.AssignedTo]; ok {
			agent.Busy = false
			agent.Tasks = s.removeTaskFromAgent(agent, taskID)
		}
	}

	s.messages <- Message{
		From:      "swarm",
		To:        task.AssignedTo,
		Type:      MsgTaskCancel,
		Content:   "",
		Timestamp: time.Now(),
		TaskID:    taskID,
	}

	return true
}

func (s *swarm) removeTaskFromAgent(agent *AgentInfo, taskID string) []string {
	for i, t := range agent.Tasks {
		if t == taskID {
			return append(agent.Tasks[:i], agent.Tasks[i+1:]...)
		}
	}
	return agent.Tasks
}

func (s *swarm) Shutdown() {
	s.cancel()
}

func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
