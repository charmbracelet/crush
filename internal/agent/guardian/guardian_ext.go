package guardian

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/crushcl/internal/agent/messagebus"
	"github.com/charmbracelet/crushcl/internal/agent/statmachine"
)

// AgentRole 代理角色
type AgentRole string

// ============================================================================
// HeartbeatAgent 心跳代理 - 專門負責監控其他代理的健康狀態
// ============================================================================

// HeartbeatAgent 心跳代理
type HeartbeatAgent struct {
	mu  sync.RWMutex
	cfg Config

	// 代理身份
	agentID   string
	agentRole AgentRole
	agentName string

	// 組件
	messageBus   *messagebus.InMemoryMessageBus
	stateMachine *statmachine.AgentStateMachine

	// 監控的代理
	watchedAgents map[string]*WatchTarget // agentID -> 監控目標
	muWatched     sync.RWMutex

	// 事件回調
	onAgentUnhealthy func(agentID string, reason string)
	onAgentDead      func(agentID string)
	onCircuitTripped func(agentID string, reason string)

	// 生命週期
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// WatchTarget 監控目標
type WatchTarget struct {
	AgentID       string
	LastHeartbeat time.Time
	MissedBeats   int
	IsHealthy     bool
	State         statmachine.State
}

// NewHeartbeatAgent 創建心跳代理
func NewHeartbeatAgent(cfg Config, mb *messagebus.InMemoryMessageBus) *HeartbeatAgent {
	ctx, cancel := context.WithCancel(context.Background())

	ha := &HeartbeatAgent{
		cfg:           cfg,
		agentID:       "heartbeat-agent",
		agentRole:     "coordinator",
		agentName:     "Heartbeat Monitor",
		messageBus:    mb,
		stateMachine:  statmachine.NewAgentStateMachine("heartbeat-agent"),
		watchedAgents: make(map[string]*WatchTarget),
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	// 設置狀態超時
	ha.stateMachine.SetTimeouts(map[statmachine.State]time.Duration{
		statmachine.StateProcessing: 1 * time.Minute,
	})

	// 訂閱消息
	mb.Subscribe(&messagebus.Subscription{
		AgentID:    ha.agentID,
		Topics:     []messagebus.MessageType{messagebus.TypeHealthResponse, messagebus.TypeAgentRegister},
		Handler:    func(msg *messagebus.Message) { ha.handleMessage(msg) },
		BufferSize: 100,
	})

	return ha
}

// SetAgentIdentity 設置代理身份
func (ha *HeartbeatAgent) SetAgentIdentity(id, name string, role AgentRole) {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.agentID = id
	ha.agentName = name
	ha.agentRole = role
}

// SetEventCallbacks 設置事件回調
func (ha *HeartbeatAgent) SetEventCallbacks(
	onUnhealthy func(agentID, reason string),
	onDead func(agentID string),
	onCircuitTripped func(agentID, reason string),
) {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.onAgentUnhealthy = onUnhealthy
	ha.onAgentDead = onDead
	ha.onCircuitTripped = onCircuitTripped
}

// Start 啟動心跳代理
func (ha *HeartbeatAgent) Start() {
	// 觸發狀態機
	ha.stateMachine.Transition(statmachine.EventStart)

	// 啟動監控迴圈
	go ha.monitorLoop()
	go ha.healthCheckLoop()
}

// monitorLoop 監控迴圈
func (ha *HeartbeatAgent) monitorLoop() {
	stateTicker := time.NewTicker(ha.cfg.StateCheckInterval)
	defer stateTicker.Stop()

	for {
		select {
		case <-ha.ctx.Done():
			return
		case <-stateTicker.C:
			ha.checkAllAgents()
		}
	}
}

// healthCheckLoop 健康檢查迴圈
func (ha *HeartbeatAgent) healthCheckLoop() {
	ticker := time.NewTicker(ha.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ha.ctx.Done():
			return
		case <-ticker.C:
			ha.sendHealthChecks()
		}
	}
}

// sendHealthChecks 發送健康檢查
func (ha *HeartbeatAgent) sendHealthChecks() {
	ha.muWatched.RLock()
	defer ha.muWatched.RUnlock()

	for agentID := range ha.watchedAgents {
		msg := messagebus.NewPriorityMessage(
			ha.agentID,
			agentID,
			messagebus.TypeHealthCheck,
			map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
			messagebus.PriorityHigh,
		)
		ha.messageBus.Send(ha.ctx, msg)
	}
}

// checkAllAgents 檢查所有代理
func (ha *HeartbeatAgent) checkAllAgents() {
	ha.muWatched.Lock()
	defer ha.muWatched.Unlock()

	now := time.Now()
	for agentID, target := range ha.watchedAgents {
		elapsed := now.Sub(target.LastHeartbeat)

		// 檢查超時
		if elapsed > ha.cfg.HeartbeatTimeout {
			target.MissedBeats++
			target.IsHealthy = false

			// 觸發事件
			if target.MissedBeats >= ha.cfg.MissedHeartbeatsMax {
				ha.handleAgentDead(agentID)
			} else {
				ha.handleAgentUnhealthy(agentID, fmt.Sprintf("心跳超時 (%d/%d)",
					target.MissedBeats, ha.cfg.MissedHeartbeatsMax))
			}
		}
	}
}

// handleMessage 處理消息
func (ha *HeartbeatAgent) handleMessage(msg *messagebus.Message) {
	switch msg.Type {
	case messagebus.TypeHealthResponse:
		ha.handleHealthResponse(msg)
	case messagebus.TypeAgentRegister:
		ha.handleAgentRegister(msg)
	}
}

// handleHealthResponse 處理健康回覆
func (ha *HeartbeatAgent) handleHealthResponse(msg *messagebus.Message) {
	ha.muWatched.Lock()
	defer ha.muWatched.Unlock()

	from := msg.From
	if target, ok := ha.watchedAgents[from]; ok {
		target.LastHeartbeat = time.Now()
		target.MissedBeats = 0
		target.IsHealthy = true
	}
}

// handleAgentRegister 處理代理註冊
func (ha *HeartbeatAgent) handleAgentRegister(msg *messagebus.Message) {
	payload := msg.Payload.(map[string]interface{})
	agentID := payload["agent_id"].(string)

	ha.AddWatchTarget(agentID)
}

// AddWatchTarget 添加監控目標
func (ha *HeartbeatAgent) AddWatchTarget(agentID string) {
	ha.muWatched.Lock()
	defer ha.muWatched.Unlock()

	if _, ok := ha.watchedAgents[agentID]; !ok {
		ha.watchedAgents[agentID] = &WatchTarget{
			AgentID:       agentID,
			LastHeartbeat: time.Now(),
			MissedBeats:   0,
			IsHealthy:     true,
		}
	}
}

// RemoveWatchTarget 移除監控目標
func (ha *HeartbeatAgent) RemoveWatchTarget(agentID string) {
	ha.muWatched.Lock()
	defer ha.muWatched.Unlock()
	delete(ha.watchedAgents, agentID)
}

// handleAgentUnhealthy 處理代理不健康
func (ha *HeartbeatAgent) handleAgentUnhealthy(agentID, reason string) {
	ha.mu.RLock()
	cb := ha.onAgentUnhealthy
	ha.mu.RUnlock()

	if cb != nil {
		go cb(agentID, reason)
	}
}

// handleAgentDead 處理代理死亡
func (ha *HeartbeatAgent) handleAgentDead(agentID string) {
	ha.mu.RLock()
	cb := ha.onAgentDead
	ha.mu.RUnlock()

	if cb != nil {
		go cb(agentID)
	}
}

// GetWatchedAgents 獲取被監控的代理列表
func (ha *HeartbeatAgent) GetWatchedAgents() map[string]*WatchTarget {
	ha.muWatched.RLock()
	defer ha.muWatched.RUnlock()

	result := make(map[string]*WatchTarget)
	for id, target := range ha.watchedAgents {
		result[id] = target
	}
	return result
}

// GetAgentHealth 獲取單個代理健康狀態
func (ha *HeartbeatAgent) GetAgentHealth(agentID string) *WatchTarget {
	ha.muWatched.RLock()
	defer ha.muWatched.RUnlock()

	if target, ok := ha.watchedAgents[agentID]; ok {
		return target
	}
	return nil
}

// IsAgentHealthy 檢查代理是否健康
func (ha *HeartbeatAgent) IsAgentHealthy(agentID string) bool {
	ha.muWatched.RLock()
	defer ha.muWatched.RUnlock()

	if target, ok := ha.watchedAgents[agentID]; ok {
		return target.IsHealthy
	}
	return false
}

// GetStats 獲取統計
func (ha *HeartbeatAgent) GetStats() map[string]interface{} {
	ha.muWatched.RLock()
	defer ha.muWatched.RUnlock()

	healthy := 0
	unhealthy := 0
	for _, t := range ha.watchedAgents {
		if t.IsHealthy {
			healthy++
		} else {
			unhealthy++
		}
	}

	return map[string]interface{}{
		"total_watched": len(ha.watchedAgents),
		"healthy":       healthy,
		"unhealthy":     unhealthy,
		"state":         ha.stateMachine.CurrentState().Description(),
	}
}

// Shutdown 關閉
func (ha *HeartbeatAgent) Shutdown() {
	ha.cancel()
	ha.stateMachine.Transition(statmachine.EventShutdown)
	close(ha.done)
}

// ============================================================================
// CircuitBreakerAgent 熔斷代理 - 專門負責熔斷邏輯
// ============================================================================

// CircuitBreakerAgent 熔斷代理
type CircuitBreakerAgent struct {
	mu  sync.RWMutex
	cfg Config

	// 代理身份
	agentID   string
	agentRole AgentRole
	agentName string

	// 組件
	messageBus   *messagebus.InMemoryMessageBus
	stateMachine *statmachine.AgentStateMachine

	// 熔斷狀態
	circuitBreakers map[string]*CircuitBreakerState
	muCircuits      sync.RWMutex

	// 事件回調
	onCircuitOpened func(agentID, reason string)
	onCircuitClosed func(agentID string)
	onRetryAllowed  func(agentID string)

	// 生命週期
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// CircuitBreakerState 熔斷器狀態
type CircuitBreakerState struct {
	AgentID     string
	Failures    int
	LastFailure time.Time
	State       CircuitState // Closed, Open, HalfOpen
	NextTry     time.Time
	TotalTrips  int
}

// CircuitState 熔斷狀態
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// NewCircuitBreakerAgent 創建熔斷代理
func NewCircuitBreakerAgent(cfg Config, mb *messagebus.InMemoryMessageBus) *CircuitBreakerAgent {
	ctx, cancel := context.WithCancel(context.Background())

	cba := &CircuitBreakerAgent{
		cfg:             cfg,
		agentID:         "circuit-breaker-agent",
		agentRole:       "circuit_breaker",
		agentName:       "Circuit Breaker",
		messageBus:      mb,
		stateMachine:    statmachine.NewAgentStateMachine("circuit-breaker-agent"),
		circuitBreakers: make(map[string]*CircuitBreakerState),
		ctx:             ctx,
		cancel:          cancel,
		done:            make(chan struct{}),
	}

	// 訂閱消息
	mb.Subscribe(&messagebus.Subscription{
		AgentID:    cba.agentID,
		Topics:     []messagebus.MessageType{messagebus.TypeTaskResult, messagebus.TypeTaskFailed},
		Handler:    func(msg *messagebus.Message) { cba.handleMessage(msg) },
		BufferSize: 100,
	})

	return cba
}

// SetAgentIdentity 設置代理身份
func (cba *CircuitBreakerAgent) SetAgentIdentity(id, name string, role AgentRole) {
	cba.mu.Lock()
	defer cba.mu.Unlock()
	cba.agentID = id
	cba.agentName = name
	cba.agentRole = role
}

// SetEventCallbacks 設置事件回調
func (cba *CircuitBreakerAgent) SetEventCallbacks(
	onOpened func(agentID, reason string),
	onClosed func(agentID string),
	onRetry func(agentID string),
) {
	cba.mu.Lock()
	defer cba.mu.Unlock()
	cba.onCircuitOpened = onOpened
	cba.onCircuitClosed = onClosed
	cba.onRetryAllowed = onRetry
}

// Start 啟動熔斷代理
func (cba *CircuitBreakerAgent) Start() {
	cba.stateMachine.Transition(statmachine.EventStart)

	// 啟動熔斷檢查迴圈
	go cba.circuitCheckLoop()
}

// circuitCheckLoop 熔斷檢查迴圈
func (cba *CircuitBreakerAgent) circuitCheckLoop() {
	ticker := time.NewTicker(cba.cfg.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cba.ctx.Done():
			return
		case <-ticker.C:
			cba.checkCircuits()
		}
	}
}

// checkCircuits 檢查所有熔斷器
func (cba *CircuitBreakerAgent) checkCircuits() {
	cba.muCircuits.Lock()
	defer cba.muCircuits.Unlock()

	now := time.Now()
	for agentID, cb := range cba.circuitBreakers {
		if cb.State == CircuitOpen && now.After(cb.NextTry) {
			// 進入半開狀態
			cb.State = CircuitHalfOpen
			cba.triggerRetryAllowed(agentID)
		}
	}
}

// handleMessage 處理消息
func (cba *CircuitBreakerAgent) handleMessage(msg *messagebus.Message) {
	switch msg.Type {
	case messagebus.TypeTaskResult:
		cba.handleTaskResult(msg)
	case messagebus.TypeTaskFailed:
		cba.handleTaskFailed(msg)
	}
}

// handleTaskResult 處理任務結果
func (cba *CircuitBreakerAgent) handleTaskResult(msg *messagebus.Message) {
	agentID := msg.From
	cba.RecordSuccess(agentID)
}

// handleTaskFailed 處理任務失敗
func (cba *CircuitBreakerAgent) handleTaskFailed(msg *messagebus.Message) {
	agentID := msg.From
	payload := msg.Payload.(map[string]interface{})
	reason := ""
	if r, ok := payload["reason"].(string); ok {
		reason = r
	}
	cba.RecordFailure(agentID, reason)
}

// RecordFailure 記錄失敗
func (cba *CircuitBreakerAgent) RecordFailure(agentID, reason string) {
	cba.muCircuits.Lock()
	defer cba.muCircuits.Unlock()

	cb, ok := cba.circuitBreakers[agentID]
	if !ok {
		cb = &CircuitBreakerState{
			AgentID: agentID,
			State:   CircuitClosed,
		}
		cba.circuitBreakers[agentID] = cb
	}

	cb.Failures++
	cb.LastFailure = time.Now()

	// 檢查是否觸發熔斷
	if cb.Failures >= cba.cfg.CircuitBreakerThreshold && cb.State == CircuitClosed {
		cb.State = CircuitOpen
		cb.NextTry = time.Now().Add(cba.cfg.RetryDelay * time.Duration(cb.Failures))
		cb.TotalTrips++
		cba.triggerCircuitOpened(agentID, fmt.Sprintf("失敗次數達到 %d", cb.Failures))
	}
}

// RecordSuccess 記錄成功
func (cba *CircuitBreakerAgent) RecordSuccess(agentID string) {
	cba.muCircuits.Lock()
	defer cba.muCircuits.Unlock()

	cb, ok := cba.circuitBreakers[agentID]
	if !ok {
		return
	}

	if cb.State == CircuitHalfOpen {
		// 成功，關閉熔斷器
		cb.State = CircuitClosed
		cb.Failures = 0
		cba.triggerCircuitClosed(agentID)
	} else if cb.State == CircuitClosed {
		// 重置失敗計數
		cb.Failures = 0
	}
}

// IsCircuitOpen 檢查熔斷器是否打開
func (cba *CircuitBreakerAgent) IsCircuitOpen(agentID string) bool {
	cba.muCircuits.RLock()
	defer cba.muCircuits.RUnlock()

	if cb, ok := cba.circuitBreakers[agentID]; ok {
		return cb.State == CircuitOpen
	}
	return false
}

// IsCircuitHealthy 檢查熔斷器是否正常（可以處理請求）
func (cba *CircuitBreakerAgent) IsCircuitHealthy(agentID string) bool {
	cba.muCircuits.RLock()
	defer cba.muCircuits.RUnlock()

	if cb, ok := cba.circuitBreakers[agentID]; ok {
		return cb.State != CircuitOpen
	}
	return true
}

// GetCircuitState 獲取熔斷器狀態
func (cba *CircuitBreakerAgent) GetCircuitState(agentID string) CircuitState {
	cba.muCircuits.RLock()
	defer cba.muCircuits.RUnlock()

	if cb, ok := cba.circuitBreakers[agentID]; ok {
		return cb.State
	}
	return CircuitClosed
}

// GetAllCircuitStates 獲取所有熔斷器狀態
func (cba *CircuitBreakerAgent) GetAllCircuitStates() map[string]*CircuitBreakerState {
	cba.muCircuits.RLock()
	defer cba.muCircuits.RUnlock()

	result := make(map[string]*CircuitBreakerState)
	for id, cb := range cba.circuitBreakers {
		result[id] = cb
	}
	return result
}

// triggerCircuitOpened 觸發熔斷打開事件
func (cba *CircuitBreakerAgent) triggerCircuitOpened(agentID, reason string) {
	cba.mu.RLock()
	cb := cba.onCircuitOpened
	cba.mu.RUnlock()

	if cb != nil {
		go cb(agentID, reason)
	}
}

// triggerCircuitClosed 觸發熔斷關閉事件
func (cba *CircuitBreakerAgent) triggerCircuitClosed(agentID string) {
	cba.mu.RLock()
	cb := cba.onCircuitClosed
	cba.mu.RUnlock()

	if cb != nil {
		go cb(agentID)
	}
}

// triggerRetryAllowed 觸發允許重試事件
func (cba *CircuitBreakerAgent) triggerRetryAllowed(agentID string) {
	cba.mu.RLock()
	cb := cba.onRetryAllowed
	cba.mu.RUnlock()

	if cb != nil {
		go cb(agentID)
	}
}

// GetStats 獲取統計
func (cba *CircuitBreakerAgent) GetStats() map[string]interface{} {
	cba.muCircuits.RLock()
	defer cba.muCircuits.RUnlock()

	closed := 0
	open := 0
	halfOpen := 0
	totalTrips := 0

	for _, cb := range cba.circuitBreakers {
		switch cb.State {
		case CircuitClosed:
			closed++
		case CircuitOpen:
			open++
		case CircuitHalfOpen:
			halfOpen++
		}
		totalTrips += cb.TotalTrips
	}

	return map[string]interface{}{
		"total_circuits": len(cba.circuitBreakers),
		"closed":         closed,
		"open":           open,
		"half_open":      halfOpen,
		"total_trips":    totalTrips,
		"state":          cba.stateMachine.CurrentState().Description(),
	}
}

// Shutdown 關閉
func (cba *CircuitBreakerAgent) Shutdown() {
	cba.cancel()
	cba.stateMachine.Transition(statmachine.EventShutdown)
	close(cba.done)
}

// ============================================================================
// GuardianExt 增強版 Guardian - 整合所有監控 Agent
// ============================================================================

// GuardianExt 增強版 Guardian
type GuardianExt struct {
	mu  sync.RWMutex
	cfg Config

	// 子代理
	heartbeatAgent *HeartbeatAgent
	circuitBreaker *CircuitBreakerAgent

	// 組件
	messageBus *messagebus.InMemoryMessageBus

	// 任務追蹤
	taskTracking map[string]*TaskTrack
	muTasks      sync.RWMutex

	// 生命週期
	ctx    context.Context
	cancel context.CancelFunc
}

// NewGuardianExt 創建增強版 Guardian
func NewGuardianExt(cfg Config, mb *messagebus.InMemoryMessageBus) *GuardianExt {
	ctx, cancel := context.WithCancel(context.Background())

	ge := &GuardianExt{
		cfg:          cfg,
		messageBus:   mb,
		taskTracking: make(map[string]*TaskTrack),
		ctx:          ctx,
		cancel:       cancel,
	}

	// 創建子代理
	ge.heartbeatAgent = NewHeartbeatAgent(cfg, mb)
	ge.circuitBreaker = NewCircuitBreakerAgent(cfg, mb)

	// 設置事件連鎖
	ge.heartbeatAgent.SetEventCallbacks(
		ge.onAgentUnhealthy,
		ge.onAgentDead,
		nil, // 熔斷由 circuit breaker 代理處理
	)

	ge.circuitBreaker.SetEventCallbacks(
		ge.onCircuitOpened,
		ge.onCircuitClosed,
		ge.onRetryAllowed,
	)

	return ge
}

// Start 啟動 GuardianExt
func (ge *GuardianExt) Start() {
	ge.heartbeatAgent.Start()
	ge.circuitBreaker.Start()
}

// onAgentUnhealthy 代理不健康回調
func (ge *GuardianExt) onAgentUnhealthy(agentID, reason string) {
	// 發送警告消息
	msg := messagebus.NewPriorityMessage(
		"guardian",
		agentID,
		messagebus.TypeTaskCancel,
		map[string]interface{}{
			"reason": "agent_unhealthy",
			"detail": reason,
		},
		messagebus.PriorityHigh,
	)
	ge.messageBus.Send(ge.ctx, msg)
}

// onAgentDead 代理死亡回調
func (ge *GuardianExt) onAgentDead(agentID string) {
	// 取消該代理的所有任務
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	for taskID, track := range ge.taskTracking {
		if track.AgentID == agentID && track.Status == "running" {
			track.Status = "agent_dead"
			msg := messagebus.NewPriorityMessage(
				"guardian",
				agentID,
				messagebus.TypeTaskCancel,
				map[string]interface{}{
					"task_id": taskID,
					"reason":  "agent_dead",
				},
				messagebus.PriorityCritical,
			)
			ge.messageBus.Send(ge.ctx, msg)
		}
	}

	// 從心跳監控移除
	ge.heartbeatAgent.RemoveWatchTarget(agentID)
}

// onCircuitOpened 熔斷打開回調
func (ge *GuardianExt) onCircuitOpened(agentID, reason string) {
	// 取消該代理的任務
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	for _, track := range ge.taskTracking {
		if track.AgentID == agentID && track.Status == "running" {
			track.Status = "circuit_open"
		}
	}
}

// onCircuitClosed 熔斷關閉回調
func (ge *GuardianExt) onCircuitClosed(agentID string) {
	// 該代理可以重新接收任務
}

// onRetryAllowed 允許重試回調
func (ge *GuardianExt) onRetryAllowed(agentID string) {
	// 標記該代理可以重試
}

// TrackTask 追蹤任務
func (ge *GuardianExt) TrackTask(taskID, agentID string, timeout time.Duration) {
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	if timeout == 0 {
		timeout = ge.cfg.TaskTimeout
	}

	ge.taskTracking[taskID] = &TaskTrack{
		TaskID:       taskID,
		AgentID:      agentID,
		StartTime:    time.Now(),
		Timeout:      timeout,
		Status:       "running",
		LastProgress: time.Now(),
	}

	// 添加到心跳監控
	ge.heartbeatAgent.AddWatchTarget(agentID)
}

// UpdateProgress 更新進度
func (ge *GuardianExt) UpdateProgress(taskID string, progress int) {
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	if track, ok := ge.taskTracking[taskID]; ok {
		track.Progress = progress
		track.LastProgress = time.Now()
	}
}

// CompleteTask 標記任務完成
func (ge *GuardianExt) CompleteTask(taskID string) {
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	var agentID string
	if track, ok := ge.taskTracking[taskID]; ok {
		track.Status = "completed"
		agentID = track.AgentID
	}
	delete(ge.taskTracking, taskID)

	// 通知熔斷器
	if agentID != "" {
		ge.circuitBreaker.RecordSuccess(agentID)
	}
}

// CancelTask 取消任務
func (ge *GuardianExt) CancelTask(taskID string) {
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	if track, ok := ge.taskTracking[taskID]; ok {
		track.Status = "cancelled"
	}
	delete(ge.taskTracking, taskID)
}

// RecordTaskFailure 記錄任務失敗
func (ge *GuardianExt) RecordTaskFailure(taskID, reason string) {
	ge.muTasks.Lock()
	defer ge.muTasks.Unlock()

	if track, ok := ge.taskTracking[taskID]; ok {
		track.Status = "failed"
		ge.circuitBreaker.RecordFailure(track.AgentID, reason)
	}
}

// GetTaskStatus 獲取任務狀態
func (ge *GuardianExt) GetTaskStatus(taskID string) string {
	ge.muTasks.RLock()
	defer ge.muTasks.RUnlock()

	if track, ok := ge.taskTracking[taskID]; ok {
		return track.Status
	}
	return "unknown"
}

// GetAgentHealth 獲取代理健康狀態
func (ge *GuardianExt) GetAgentHealth(agentID string) *WatchTarget {
	return ge.heartbeatAgent.GetAgentHealth(agentID)
}

// IsAgentHealthy 檢查代理是否健康
func (ge *GuardianExt) IsAgentHealthy(agentID string) bool {
	if !ge.heartbeatAgent.IsAgentHealthy(agentID) {
		return false
	}
	if !ge.circuitBreaker.IsCircuitHealthy(agentID) {
		return false
	}
	return true
}

// GetStats 獲取統計
func (ge *GuardianExt) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"heartbeat":       ge.heartbeatAgent.GetStats(),
		"circuit_breaker": ge.circuitBreaker.GetStats(),
		"tasks":           ge.getTaskStats(),
	}
}

// getTaskStats 獲取任務統計
func (ge *GuardianExt) getTaskStats() map[string]interface{} {
	ge.muTasks.RLock()
	defer ge.muTasks.RUnlock()

	running := 0
	completed := 0
	failed := 0

	for _, track := range ge.taskTracking {
		switch track.Status {
		case "running":
			running++
		case "completed":
			completed++
		case "failed", "agent_dead", "circuit_open":
			failed++
		}
	}

	return map[string]interface{}{
		"total":     len(ge.taskTracking),
		"running":   running,
		"completed": completed,
		"failed":    failed,
	}
}

// Shutdown 關閉
func (ge *GuardianExt) Shutdown() {
	ge.cancel()
	ge.heartbeatAgent.Shutdown()
	ge.circuitBreaker.Shutdown()
}
