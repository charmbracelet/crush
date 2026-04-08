package guardian

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/crushcl/internal/agent/messagebus"
	"github.com/charmbracelet/crushcl/internal/agent/statmachine"
)

// ============================================================================
// Guardian 配置
// ============================================================================

// Config Guardian 配置
type Config struct {
	// 心跳配置
	HeartbeatInterval   time.Duration // 發送心跳間隔
	HeartbeatTimeout    time.Duration // 心跳超時
	MissedHeartbeatsMax int           // 最大丟失心跳數

	// 任務超時
	TaskTimeout        time.Duration            // 任務默認超時
	TaskTimeoutPerType map[string]time.Duration // 按任務類型自定義超時

	// 重試配置
	MaxRetries              int           // 最大重試次數
	RetryDelay              time.Duration // 重試延遲
	CircuitBreakerThreshold int           // 熔斷閾值

	// 監控配置
	MonitorInterval    time.Duration // 監控檢查間隔
	StateCheckInterval time.Duration // 狀態檢查間隔

	// 死鎖檢測
	DeadlockDetection     bool          // 是否啟用死鎖檢測
	DeadlockTimeout       time.Duration // 死鎖超時
	DeadlockCheckInterval time.Duration // 死鎖檢查間隔
}

// DefaultConfig 返回默認配置
func DefaultConfig() Config {
	return Config{
		HeartbeatInterval:       5 * time.Second,
		HeartbeatTimeout:        15 * time.Second,
		MissedHeartbeatsMax:     3,
		TaskTimeout:             5 * time.Minute,
		TaskTimeoutPerType:      make(map[string]time.Duration),
		MaxRetries:              3,
		RetryDelay:              2 * time.Second,
		CircuitBreakerThreshold: 5,
		MonitorInterval:         1 * time.Second,
		StateCheckInterval:      10 * time.Second,
		DeadlockDetection:       true,
		DeadlockTimeout:         30 * time.Second,
		DeadlockCheckInterval:   5 * time.Second,
	}
}

// ============================================================================
// Agent Status
// ============================================================================

// AgentStatus 代理狀態
type AgentStatus struct {
	AgentID       string
	State         statmachine.State
	LastHeartbeat time.Time
	MissedBeats   int
	CurrentTask   string
	Retries       int
	IsBlocked     bool
	IsCircuitOpen bool
	ErrorMessage  string
}

// ============================================================================
// Guardian 守護者
// ============================================================================

// Guardian Agent 守護者 - 防止卡住和中断
type Guardian struct {
	mu  sync.RWMutex
	cfg Config

	// 組件
	messageBus *messagebus.InMemoryMessageBus
	agents     map[string]*AgentStatus
	heartbeats map[string]chan struct{} // agentID -> heartbeat channel

	// 熔斷器
	circuitBreaker map[string]*CircuitBreaker

	// 死鎖檢測
	deadlockDetector *DeadlockDetector

	// 任務追蹤
	taskTracking map[string]*TaskTrack

	// 監控
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// TaskTrack 任務追蹤
type TaskTrack struct {
	TaskID       string
	AgentID      string
	StartTime    time.Time
	Timeout      time.Duration
	Status       string
	LastProgress time.Time
	Progress     int
}

// CircuitBreaker 熔斷器
type CircuitBreaker struct {
	failures    int
	lastFailure time.Time
	isOpen      bool
	nextTry     time.Time
}

// NewGuardian 創建新的 Guardian
func NewGuardian(cfg Config, mb *messagebus.InMemoryMessageBus) *Guardian {
	ctx, cancel := context.WithCancel(context.Background())
	g := &Guardian{
		cfg:              cfg,
		messageBus:       mb,
		agents:           make(map[string]*AgentStatus),
		heartbeats:       make(map[string]chan struct{}),
		circuitBreaker:   make(map[string]*CircuitBreaker),
		deadlockDetector: NewDeadlockDetector(cfg.DeadlockTimeout),
		taskTracking:     make(map[string]*TaskTrack),
		ctx:              ctx,
		cancel:           cancel,
		done:             make(chan struct{}),
	}

	return g
}

// RegisterAgent 註冊代理
func (g *Guardian) RegisterAgent(agentID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.agents[agentID] = &AgentStatus{
		AgentID:       agentID,
		State:         statmachine.StateBooting,
		LastHeartbeat: time.Now(),
		MissedBeats:   0,
		Retries:       0,
	}

	g.heartbeats[agentID] = make(chan struct{}, 1)
	g.circuitBreaker[agentID] = &CircuitBreaker{}
}

// UnregisterAgent 取消註冊代理
func (g *Guardian) UnregisterAgent(agentID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.agents, agentID)
	if ch, ok := g.heartbeats[agentID]; ok {
		close(ch)
		delete(g.heartbeats, agentID)
	}
	delete(g.circuitBreaker, agentID)
}

// StartHeartbeat 啟動心跳
func (g *Guardian) StartHeartbeat(agentID string) {
	g.mu.RLock()
	ch, ok := g.heartbeats[agentID]
	g.mu.RUnlock()

	if !ok {
		return
	}

	go func() {
		ticker := time.NewTicker(g.cfg.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-g.ctx.Done():
				return
			case <-ch:
				return
			case <-ticker.C:
				// 發送心跳消息
				msg := messagebus.NewMessage("guardian", agentID, messagebus.TypeHealthCheck, nil)
				msg.Priority = messagebus.PriorityHigh
				g.messageBus.Send(g.ctx, msg)
			}
		}
	}()
}

// StopHeartbeat 停止心跳
func (g *Guardian) StopHeartbeat(agentID string) {
	g.mu.RLock()
	ch, ok := g.heartbeats[agentID]
	g.mu.RUnlock()

	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// ReceiveHeartbeat 接收心跳回覆
func (g *Guardian) ReceiveHeartbeat(agentID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if status, ok := g.agents[agentID]; ok {
		status.LastHeartbeat = time.Now()
		status.MissedBeats = 0
	}
}

// TrackTask 追蹤任務
func (g *Guardian) TrackTask(taskID, agentID string, timeout time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if timeout == 0 {
		timeout = g.cfg.TaskTimeout
	}

	g.taskTracking[taskID] = &TaskTrack{
		TaskID:       taskID,
		AgentID:      agentID,
		StartTime:    time.Now(),
		Timeout:      timeout,
		Status:       "running",
		LastProgress: time.Now(),
	}
}

// UpdateProgress 更新進度
func (g *Guardian) UpdateProgress(taskID string, progress int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if track, ok := g.taskTracking[taskID]; ok {
		track.Progress = progress
		track.LastProgress = time.Now()
	}
}

// CompleteTask 標記任務完成
func (g *Guardian) CompleteTask(taskID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if track, ok := g.taskTracking[taskID]; ok {
		track.Status = "completed"
	}
	delete(g.taskTracking, taskID)
}

// CancelTask 取消任務
func (g *Guardian) CancelTask(taskID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if track, ok := g.taskTracking[taskID]; ok {
		track.Status = "cancelled"
	}
	delete(g.taskTracking, taskID)
}

// GetAgentStatus 獲取代理狀態
func (g *Guardian) GetAgentStatus(agentID string) *AgentStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if status, ok := g.agents[agentID]; ok {
		return status
	}
	return nil
}

// GetAllStatus 獲取所有代理狀態
func (g *Guardian) GetAllStatus() map[string]*AgentStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]*AgentStatus)
	for id, status := range g.agents {
		result[id] = status
	}
	return result
}

// IsAgentHealthy 檢查代理是否健康
func (g *Guardian) IsAgentHealthy(agentID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	status, ok := g.agents[agentID]
	if !ok {
		return false
	}

	// 檢查心跳
	if time.Since(status.LastHeartbeat) > g.cfg.HeartbeatTimeout {
		return false
	}

	// 檢查熔斷
	if cb, ok := g.circuitBreaker[agentID]; ok && cb.isOpen {
		return false
	}

	return true
}

// Start 啟動 Guardian
func (g *Guardian) Start() {
	go g.monitorLoop()
	go g.deadlockLoop()
}

// monitorLoop 監控迴圈
func (g *Guardian) monitorLoop() {
	ticker := time.NewTicker(g.cfg.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.checkAgents()
			g.checkTasks()
		}
	}
}

// checkAgents 檢查所有代理
func (g *Guardian) checkAgents() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for agentID, status := range g.agents {
		// 檢查心跳超時
		if now.Sub(status.LastHeartbeat) > g.cfg.HeartbeatTimeout {
			status.MissedBeats++
			if status.MissedBeats >= g.cfg.MissedHeartbeatsMax {
				g.handleAgentTimeout(agentID)
			}
		}

		// 檢查熔斷器
		if cb, ok := g.circuitBreaker[agentID]; ok {
			if cb.isOpen && now.After(cb.nextTry) {
				cb.isOpen = false
				status.IsCircuitOpen = false
			}
		}
	}
}

// checkTasks 檢查所有任務
func (g *Guardian) checkTasks() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for taskID, track := range g.taskTracking {
		if track.Status != "running" {
			continue
		}

		// 檢查任務超時
		if now.Sub(track.StartTime) > track.Timeout {
			g.handleTaskTimeout(taskID)
			continue
		}

		// 檢查進度停滯（可能是死鎖）
		if g.cfg.DeadlockDetection && now.Sub(track.LastProgress) > g.cfg.DeadlockTimeout {
			g.deadlockDetector.ReportPotential(taskID, track.AgentID)
		}
	}
}

// handleAgentTimeout 處理代理超時
func (g *Guardian) handleAgentTimeout(agentID string) {
	status := g.agents[agentID]
	status.IsBlocked = true
	status.ErrorMessage = fmt.Sprintf("心跳超時，已丟失 %d 個心跳", status.MissedBeats)

	// 發送重試或取消任務
	msg := messagebus.NewMessage("guardian", agentID, messagebus.TypeTaskCancel, map[string]interface{}{
		"reason": "heartbeat_timeout",
	})
	msg.Priority = messagebus.PriorityCritical
	g.messageBus.Send(g.ctx, msg)
}

// handleTaskTimeout 處理任務超時
func (g *Guardian) handleTaskTimeout(taskID string) {
	track := g.taskTracking[taskID]
	if track == nil {
		return
	}

	track.Status = "timeout"
	status := g.agents[track.AgentID]
	if status != nil {
		status.Retries++
		if status.Retries >= g.cfg.MaxRetries {
			// 熔斷
			g.tripCircuitBreaker(track.AgentID, fmt.Sprintf("任務 %s 達到最大重試次數", taskID))
		}
	}

	// 發送超時消息
	msg := messagebus.NewMessage("guardian", track.AgentID, messagebus.TypeTaskCancel, map[string]interface{}{
		"task_id": taskID,
		"reason":  "task_timeout",
	})
	msg.Priority = messagebus.PriorityCritical
	g.messageBus.Send(g.ctx, msg)
}

// tripCircuitBreaker 觸發熔斷
func (g *Guardian) tripCircuitBreaker(agentID, reason string) {
	cb := g.circuitBreaker[agentID]
	cb.isOpen = true
	cb.nextTry = time.Now().Add(g.cfg.RetryDelay * time.Duration(g.cfg.CircuitBreakerThreshold))

	status := g.agents[agentID]
	if status != nil {
		status.IsCircuitOpen = true
		status.ErrorMessage = reason
	}
}

// RecordFailure 記錄失敗
func (g *Guardian) RecordFailure(agentID string, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	cb := g.circuitBreaker[agentID]
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= g.cfg.CircuitBreakerThreshold {
		cb.isOpen = true
		cb.nextTry = time.Now().Add(g.cfg.RetryDelay * time.Duration(cb.failures))

		status := g.agents[agentID]
		if status != nil {
			status.IsCircuitOpen = true
			status.ErrorMessage = fmt.Sprintf("熔斷打開: %v", err)
		}
	}
}

// RecordSuccess 記錄成功
func (g *Guardian) RecordSuccess(agentID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	cb := g.circuitBreaker[agentID]
	cb.failures = 0
	cb.isOpen = false

	status := g.agents[agentID]
	if status != nil {
		status.Retries = 0
		status.IsCircuitOpen = false
	}
}

// deadlockLoop 死鎖檢測迴圈
func (g *Guardian) deadlockLoop() {
	if !g.cfg.DeadlockDetection {
		return
	}

	ticker := time.NewTicker(g.cfg.DeadlockCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.checkDeadlock()
		}
	}
}

// checkDeadlock 檢查死鎖
func (g *Guardian) checkDeadlock() {
	deadlocked := g.deadlockDetector.Detect()
	for taskID, agentID := range deadlocked {
		g.handleDeadlock(taskID, agentID)
	}
}

// handleDeadlock 處理死鎖
func (g *Guardian) handleDeadlock(taskID, agentID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	status := g.agents[agentID]
	if status != nil {
		status.IsBlocked = true
		status.ErrorMessage = fmt.Sprintf("檢測到任務 %s 死鎖", taskID)
	}

	// 取消任務
	msg := messagebus.NewMessage("guardian", agentID, messagebus.TypeTaskCancel, map[string]interface{}{
		"task_id": taskID,
		"reason":  "deadlock",
	})
	msg.Priority = messagebus.PriorityCritical
	g.messageBus.Send(g.ctx, msg)
}

// Shutdown 關閉 Guardian
func (g *Guardian) Shutdown() {
	g.cancel()
	close(g.done)
}

// ============================================================================
// Deadlock Detector 死鎖檢測器
// ============================================================================

// DeadlockDetector 死鎖檢測器
type DeadlockDetector struct {
	mu        sync.Mutex
	waitTimes map[string]time.Time // taskID -> 首次報告時間
	threshold time.Duration
}

// NewDeadlockDetector 創建死鎖檢測器
func NewDeadlockDetector(threshold time.Duration) *DeadlockDetector {
	return &DeadlockDetector{
		waitTimes: make(map[string]time.Time),
		threshold: threshold,
	}
}

// ReportPotential 報告可能的死鎖
func (dd *DeadlockDetector) ReportPotential(taskID, agentID string) {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	if _, ok := dd.waitTimes[taskID]; !ok {
		dd.waitTimes[taskID] = time.Now()
	}
}

// Detect 檢測死鎖
func (dd *DeadlockDetector) Detect() map[string]string {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	result := make(map[string]string)
	now := time.Now()

	for taskID, start := range dd.waitTimes {
		if now.Sub(start) > dd.threshold {
			result[taskID] = "" // taskID -> agentID (需要從外面查)
		}
	}

	// 清理舊記錄
	dd.waitTimes = make(map[string]time.Time)

	return result
}
