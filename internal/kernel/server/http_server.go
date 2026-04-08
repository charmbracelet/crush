package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/crushcl/internal/kernel"
	"github.com/charmbracelet/crushcl/internal/kernel/cl_kernel"
	"github.com/charmbracelet/crushcl/internal/kernel/coordination"
	"github.com/charmbracelet/crushcl/internal/session"
)

// ServerConfig 伺服器配置
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultServerConfig 返回預設配置
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// Server HTTP API 伺服器
type Server struct {
	config  ServerConfig
	mux     *http.ServeMux
	server  *http.Server
	handler *APIHandler
}

// NewServer 創建新的伺服器
func NewServer(config ...ServerConfig) *Server {
	cfg := DefaultServerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	mux := http.NewServeMux()
	s := &Server{
		config:  cfg,
		mux:     mux,
		handler: NewAPIHandler(),
	}

	s.setupRoutes()
	return s
}

// NewServerWithAgent 創建帶有 Agent Runner 的伺服器
// 這確保真實的 MiniMax API 被調用，避免執行本地 mock
func NewServerWithAgent(runner cl_kernel.AgentRunner, config ...ServerConfig) *Server {
	cfg := DefaultServerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	mux := http.NewServeMux()
	s := &Server{
		config:  cfg,
		mux:     mux,
		handler: NewAPIHandlerWithAgent(runner),
	}

	s.setupRoutes()
	return s
}

// setupRoutes 設置路由
func (s *Server) setupRoutes() {
	// 健康檢查
	s.mux.HandleFunc("/health", s.handler.handleHealth)

	// API v1
	api := "/api/v1"

	// 執行
	s.mux.HandleFunc(api+"/execute", s.handler.handleExecute)
	s.mux.HandleFunc(api+"/execute/stream", s.handler.handleExecuteStream)

	// 任務
	s.mux.HandleFunc(api+"/tasks", s.handler.handleListTasks)
	s.mux.HandleFunc(api+"/tasks/", s.handler.handleTaskByID)

	// 會話
	s.mux.HandleFunc(api+"/sessions", s.handler.handleListSessions)
	s.mux.HandleFunc(api+"/sessions/", s.handler.handleSessionByID)

	// 預算
	s.mux.HandleFunc(api+"/budget", s.handler.handleGetBudget)
	s.mux.HandleFunc(api+"/budget/reset", s.handler.handleResetBudget)

	// 協調器
	s.mux.HandleFunc(api+"/coordination/stats", s.handler.handleCoordinationStats)
}

// Start 啟動伺服器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	return s.server.ListenAndServe()
}

// StartTLS 啟動 HTTPS 伺服器
func (s *Server) StartTLS(certFile, keyFile string) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	return s.server.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown 優雅關閉
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// ============================================================================
// HTTP 處理器
// ============================================================================

// APIHandler API 處理器
type APIHandler struct {
	hybridBrain   coordination.HybridBrain     // HybridBrain 介面
	sessionSvc    session.Service              // Session 服務（介面）
	usageTracker  *kernel.EnhancedUsageTracker // Usage 追蹤器
	taskScheduler *coordination.TaskScheduler  // 任務調度器（可選）
}

// NewAPIHandler 創建新的 API 處理器（使用預設值）
func NewAPIHandler() *APIHandler {
	return &APIHandler{
		hybridBrain:  coordination.NewHybridBrain(),
		sessionSvc:   nil, // 需要透過 NewAPIHandlerWithDeps 設置
		usageTracker: kernel.NewEnhancedUsageTracker(),
	}
}

// NewAPIHandlerWithBrain 創建帶有指定 HybridBrain 的 API 處理器
func NewAPIHandlerWithBrain(brain coordination.HybridBrain) *APIHandler {
	return &APIHandler{
		hybridBrain:  brain,
		sessionSvc:   nil,
		usageTracker: kernel.NewEnhancedUsageTracker(),
	}
}

// NewAPIHandlerWithDeps 創建帶有完整依賴的 API 處理器
func NewAPIHandlerWithDeps(brain coordination.HybridBrain, sessionSvc session.Service, usageTracker *kernel.EnhancedUsageTracker) *APIHandler {
	return &APIHandler{
		hybridBrain:  brain,
		sessionSvc:   sessionSvc,
		usageTracker: usageTracker,
	}
}

// NewAPIHandlerWithAgent 創建帶有 Agent Runner 的 API 處理器
// 這是推薦的工廠函數，可以確保真實的 MiniMax API 被調用
func NewAPIHandlerWithAgent(runner cl_kernel.AgentRunner) *APIHandler {
	brain := coordination.NewHybridBrainWithAgent(runner)
	return &APIHandler{
		hybridBrain:  brain,
		sessionSvc:   nil,
		usageTracker: kernel.NewEnhancedUsageTracker(),
	}
}

// SetSessionService 設置 session 服務
func (h *APIHandler) SetSessionService(svc session.Service) {
	h.sessionSvc = svc
}

// SetUsageTracker 設置 usage 追蹤器
func (h *APIHandler) SetUsageTracker(tracker *kernel.EnhancedUsageTracker) {
	h.usageTracker = tracker
}

// SetTaskScheduler 設置任務調度器
func (h *APIHandler) SetTaskScheduler(scheduler *coordination.TaskScheduler) {
	h.taskScheduler = scheduler
}

// handleHealth 健康檢查
func (h *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// Execute API
// ============================================================================

// ExecuteRequest 執行請求
type ExecuteRequest struct {
	Prompt   string   `json:"prompt"`
	Tools    []string `json:"tools,omitempty"`
	Executor string   `json:"executor,omitempty"` // auto, cl, cc, hybrid
	Model    string   `json:"model,omitempty"`
	Stream   bool     `json:"stream,omitempty"`
}

// ExecuteResponse 執行回應
type ExecuteResponse struct {
	SessionID  string  `json:"session_id"`
	Text       string  `json:"text"`
	Tokens     int     `json:"tokens"`
	CostUSD    float64 `json:"cost_usd"`
	Executor   string  `json:"executor"`
	DurationMs int64   `json:"duration_ms"`
}

func (h *APIHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	result := h.executeTask(ctx, req)

	writeJSON(w, http.StatusOK, ExecuteResponse{
		SessionID:  generateSessionID(),
		Text:       result.Output,
		Tokens:     result.Tokens,
		CostUSD:    result.Cost,
		Executor:   string(result.Executor),
		DurationMs: result.Duration.Milliseconds(),
	})
}

func (h *APIHandler) handleExecuteStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 設置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSE(w, fmt.Sprintf("data: {\"type\": \"error\", \"error\": \"%v\"}\n\n", err))
		return
	}

	ctx := r.Context()
	sessionID := generateSessionID()

	// 發送開始事件
	writeSSE(w, fmt.Sprintf("data: {\"type\": \"started\", \"session_id\": \"%s\", \"prompt\": \"%s\"}\n\n",
		sessionID, escapeForSSE(req.Prompt)))

	// 執行任務（帶超時）
	executeCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// 使用單獨的 goroutine 執行任務並發送進度更新
	done := make(chan struct{})
	go func() {
		defer close(done)

		// 發送分類事件
		if h.hybridBrain != nil {
			classification := h.hybridBrain.ClassifyTask(req.Prompt)
			writeSSE(w, fmt.Sprintf("data: {\"type\": \"classified\", \"task_type\": \"%s\", \"executor\": \"%s\", \"confidence\": %.2f, \"cost_estimate\": %.4f}\n\n",
				classification.TaskType, classification.Executor, classification.Confidence, classification.CostEstimate))
		}

		// 執行任務
		brainReq := coordination.ExecuteRequest{
			Prompt:    req.Prompt,
			Tools:     req.Tools,
			Executor:  coordination.ExecutorType(req.Executor),
			Model:     req.Model,
			Stream:    true,
			SessionID: sessionID,
		}

		result := h.hybridBrain.Execute(executeCtx, brainReq)

		// 發送完成事件
		writeSSE(w, fmt.Sprintf("data: {\"type\": \"result\", \"session_id\": \"%s\", \"output\": %s, \"tokens\": %d, \"cost_usd\": %.6f, \"executor\": \"%s\", \"duration_ms\": %d}\n\n",
			sessionID,
			escapeForSSE(result.Output),
			result.Tokens,
			result.CostUSD,
			result.Executor,
			result.Duration.Milliseconds()))
	}()

	// 等待完成或客戶端斷開
	select {
	case <-done:
		writeSSE(w, "data: {\"type\": \"done\"}\n\n")
	case <-r.Context().Done():
		writeSSE(w, "data: {\"type\": \"cancelled\"}\n\n")
	}
}

// escapeForSSE 轉義字串以適合 SSE data 字段
func escapeForSSE(s string) string {
	// 轉義換行、 carriage return 和引號
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// ============================================================================
// Tasks API
// ============================================================================

func (h *APIHandler) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if h.taskScheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "Task scheduler not configured")
		return
	}

	// 獲取運行中的任務
	runningTasks := h.taskScheduler.GetRunningTasks()
	historyTasks := h.taskScheduler.GetHistory(100) // 最近 100 個歷史任務
	stats := h.taskScheduler.GetStats()

	// 轉換運行中的任務
	runningList := make([]map[string]interface{}, len(runningTasks))
	for i, task := range runningTasks {
		runningList[i] = map[string]interface{}{
			"id":         task.ID,
			"prompt":     task.Prompt,
			"priority":   task.Priority.String(),
			"state":      "running",
			"executor":   task.Executor,
			"created_at": task.CreatedAt.Format(time.RFC3339),
			"started_at": task.StartedAt.Format(time.RFC3339),
		}
	}

	// 轉換歷史任務
	historyList := make([]map[string]interface{}, len(historyTasks))
	for i, task := range historyTasks {
		stateStr := "unknown"
		switch task.State {
		case coordination.TaskStateCompleted:
			stateStr = "completed"
		case coordination.TaskStateFailed:
			stateStr = "failed"
		case coordination.TaskStateCancelled:
			stateStr = "cancelled"
		}

		taskMap := map[string]interface{}{
			"id":           task.ID,
			"prompt":       task.Prompt,
			"priority":     task.Priority.String(),
			"state":        stateStr,
			"executor":     task.Executor,
			"created_at":   task.CreatedAt.Format(time.RFC3339),
			"completed_at": task.CompletedAt.Format(time.RFC3339),
		}
		if task.Result != nil {
			taskMap["tokens"] = task.Result.Tokens
			taskMap["cost_usd"] = task.Result.CostUSD
		}
		historyList[i] = taskMap
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running_tasks": runningList,
		"history_tasks": historyList,
		"stats":         stats,
		"total_running": len(runningList),
		"total_history": len(historyList),
	})
}

func (h *APIHandler) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	taskID := extractPathParam(r.URL.Path, "/api/v1/tasks/")

	if taskID == "" {
		writeError(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	if h.taskScheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "Task scheduler not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 獲取任務詳情
		task := h.taskScheduler.GetTask(taskID)
		if task == nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("Task not found: %s", taskID))
			return
		}

		stateStr := "pending"
		switch task.State {
		case coordination.TaskStatePending:
			stateStr = "pending"
		case coordination.TaskStateRunning:
			stateStr = "running"
		case coordination.TaskStateCompleted:
			stateStr = "completed"
		case coordination.TaskStateFailed:
			stateStr = "failed"
		case coordination.TaskStateCancelled:
			stateStr = "cancelled"
		}

		taskMap := map[string]interface{}{
			"id":         task.ID,
			"prompt":     task.Prompt,
			"priority":   task.Priority.String(),
			"state":      stateStr,
			"executor":   task.Executor,
			"tools":      task.Tools,
			"created_at": task.CreatedAt.Format(time.RFC3339),
		}

		if !task.StartedAt.IsZero() {
			taskMap["started_at"] = task.StartedAt.Format(time.RFC3339)
		}
		if !task.CompletedAt.IsZero() {
			taskMap["completed_at"] = task.CompletedAt.Format(time.RFC3339)
		}
		if task.Result != nil {
			taskMap["result"] = map[string]interface{}{
				"output":        task.Result.Output,
				"tokens":        task.Result.Tokens,
				"cost_usd":      task.Result.CostUSD,
				"duration_ms":   task.Result.Duration.Milliseconds(),
				"cache_hit":     task.Result.CacheHit,
				"executor_used": task.Result.ExecutorUsed,
			}
		}
		if task.Error != nil {
			taskMap["error"] = task.Error.Error()
		}

		writeJSON(w, http.StatusOK, taskMap)

	case http.MethodDelete:
		// 取消任務
		cancelled := h.taskScheduler.CancelTask(taskID)
		if !cancelled {
			writeError(w, http.StatusNotFound, fmt.Sprintf("Task not found or already completed: %s", taskID))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cancelled": true,
			"task_id":   taskID,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ============================================================================
// Sessions API
// ============================================================================

func (h *APIHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	// 如果有 session service，使用它
	if h.sessionSvc != nil {
		sessions, err := h.sessionSvc.List(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list sessions: %v", err))
			return
		}

		// 轉換為 API 格式
		sessionList := make([]map[string]interface{}, len(sessions))
		for i, s := range sessions {
			sessionList[i] = map[string]interface{}{
				"id":                s.ID,
				"title":             s.Title,
				"message_count":     s.MessageCount,
				"prompt_tokens":     s.PromptTokens,
				"completion_tokens": s.CompletionTokens,
				"cost":              s.Cost,
				"created_at":        s.CreatedAt,
				"updated_at":        s.UpdatedAt,
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sessions": sessionList,
			"total":    len(sessionList),
		})
		return
	}

	// 沒有 session service，返回空列表（向後兼容）
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": []interface{}{},
		"total":    0,
		"note":     "Session service not configured",
	})
}

func (h *APIHandler) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	sessionID := extractPathParam(r.URL.Path, "/api/v1/sessions/")

	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// 獲取會話詳情
		if h.sessionSvc != nil {
			sess, err := h.sessionSvc.Get(ctx, sessionID)
			if err != nil {
				writeError(w, http.StatusNotFound, fmt.Sprintf("Session not found: %v", err))
				return
			}

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":                sess.ID,
				"parent_session_id": sess.ParentSessionID,
				"title":             sess.Title,
				"message_count":     sess.MessageCount,
				"prompt_tokens":     sess.PromptTokens,
				"completion_tokens": sess.CompletionTokens,
				"cost":              sess.Cost,
				"todos":             sess.Todos,
				"created_at":        sess.CreatedAt,
				"updated_at":        sess.UpdatedAt,
			})
			return
		}

		// 沒有 session service，返回基本信息
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": sessionID,
			"note":       "Session service not configured",
		})

	case http.MethodDelete:
		// 刪除會話
		if h.sessionSvc != nil {
			err := h.sessionSvc.Delete(ctx, sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete session: %v", err))
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"deleted":    true,
				"session_id": sessionID,
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deleted":    true,
			"session_id": sessionID,
			"note":       "Session service not configured",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ============================================================================
// Budget API
// ============================================================================

func (h *APIHandler) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 使用 usage tracker 获取预算状态
	if h.usageTracker != nil {
		budgets := h.usageTracker.GetBudgetStatus()
		stats := h.usageTracker.GetGlobalStats()

		budgetList := make([]map[string]interface{}, 0, len(budgets))
		for name, budget := range budgets {
			budgetList = append(budgetList, map[string]interface{}{
				"name":          name,
				"max_tokens":    budget.MaxTokens,
				"max_cost":      budget.MaxCost,
				"current_usage": budget.CurrentUsage,
				"current_cost":  budget.CurrentCost,
				"reset_time":    budget.ResetTime.Format(time.RFC3339),
				"alert_at":      budget.AlertAt,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"global_stats": map[string]interface{}{
				"total_sessions":      stats.TotalSessions,
				"active_sessions":     stats.ActiveSessions,
				"total_input_tokens":  stats.TotalInputTokens,
				"total_output_tokens": stats.TotalOutputTokens,
				"total_cache_tokens":  stats.TotalCacheTokens,
				"total_cost":          stats.TotalCost,
				"total_tool_calls":    stats.TotalToolCalls,
				"total_compactions":   stats.TotalCompactions,
			},
			"budgets":       budgetList,
			"total_budgets": len(budgetList),
		})
		return
	}

	// 沒有 usage tracker，返回預設值
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"max_budget_usd":   10.0,
		"used_budget_usd":  0.0,
		"remaining_budget": 10.0,
		"note":             "Usage tracker not configured",
	})
}

func (h *APIHandler) handleResetBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 解析請求體獲取預算名稱
	var req struct {
		BudgetName string `json:"budget_name,omitempty"`
	}

	// 解析請求體，忽略空 body 錯誤
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	if h.usageTracker != nil {
		if req.BudgetName != "" {
			// 重置特定預算
			budgets := h.usageTracker.GetBudgetStatus()
			if budget, ok := budgets[req.BudgetName]; ok {
				budget.CurrentUsage = 0
				budget.CurrentCost = 0
				budget.ResetTime = time.Now().Add(24 * time.Hour)
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"reset":       true,
					"budget_name": req.BudgetName,
				})
				return
			}
			writeError(w, http.StatusNotFound, fmt.Sprintf("Budget not found: %s", req.BudgetName))
			return
		}

		// 重置所有預算
		budgets := h.usageTracker.GetBudgetStatus()
		for _, budget := range budgets {
			budget.CurrentUsage = 0
			budget.CurrentCost = 0
			budget.ResetTime = time.Now().Add(24 * time.Hour)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"reset":        true,
			"all_budgets":  true,
			"budget_count": len(budgets),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reset": true,
		"note":  "Usage tracker not configured",
	})
}

// ============================================================================
// Coordination API
// ============================================================================

func (h *APIHandler) handleCoordinationStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 從 HybridBrain 獲取統計
	if h.hybridBrain != nil {
		stats := h.hybridBrain.GetStats()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"hybrid_brain": map[string]interface{}{
				"used_budget_usd":  stats.UsedBudgetUSD,
				"max_budget_usd":   stats.MaxBudgetUSD,
				"remaining_budget": stats.RemainingBudget,
				"total_tokens":     stats.TotalTokens,
				"tasks_executed":   stats.TasksExecuted,
				"budget_warning":   stats.BudgetWarning,
			},
			"usage_tracker": func() map[string]interface{} {
				if h.usageTracker != nil {
					metrics := h.usageTracker.Metrics()
					return metrics
				}
				return nil
			}(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"error": "HybridBrain not configured",
	})
}

// ============================================================================
// 輔助函數
// ============================================================================

// TaskResult 任務結果（與 HybridBrain 保持一致）
type TaskResult struct {
	Output   string
	Executor string
	Tokens   int
	Cost     float64
	Duration time.Duration
}

func (h *APIHandler) executeTask(ctx context.Context, req ExecuteRequest) *TaskResult {
	// 使用 HybridBrain.Execute() 執行任務
	brainReq := coordination.ExecuteRequest{
		Prompt:   req.Prompt,
		Tools:    req.Tools,
		Executor: coordination.ExecutorType(req.Executor),
		Model:    req.Model,
		Stream:   req.Stream,
	}

	brainResult := h.hybridBrain.Execute(ctx, brainReq)

	return &TaskResult{
		Output:   brainResult.Output,
		Executor: string(brainResult.Executor),
		Tokens:   brainResult.Tokens,
		Cost:     brainResult.CostUSD,
		Duration: brainResult.Duration,
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

func writeSSE(w http.ResponseWriter, data string) {
	fmt.Fprint(w, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func extractPathParam(path, prefix string) string {
	if len(path) > len(prefix) {
		return path[len(prefix):]
	}
	return ""
}

func generateSessionID() string {
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}
