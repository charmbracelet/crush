// Copyright 2026 CrushCL. All rights reserved.
//
// HybridBrain Interface - 定義 HybridBrain 的標準介面
// 由 Crush 作為大腦統一調度 CL Native 和 Claude Code

package coordination

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/crushcl/internal/kernel/cl_kernel"
	"github.com/google/uuid"
)

// HybridBrain 是 CrushCL 混合智能控制器的核心介面
// 負責：
// 1. 任務分類
// 2. 路由決策
// 3. CL/Claude Code 調度
// 4. 成本控制
// 5. 結果整合
type HybridBrain interface {
	// Think 是大腦的核心方法 - 分析並決策
	Think(ctx context.Context, task string, forcedExecutor ...ExecutorType) *BrainResult

	// Execute 執行任務並返回結果
	Execute(ctx context.Context, req ExecuteRequest) *BrainResult

	// GetStats 返回當前統計
	GetStats() BrainStats

	// ClassifyTask 對任務進行分類
	ClassifyTask(task string) TaskClassification

	// SetAgentRunner 設置 Agent Runner 以啟用真實 Agent 執行
	// 這允許 HybridBrain 使用 SessionAgent 作為執行者
	SetAgentRunner(runner cl_kernel.AgentRunner)
}

// BrainResult 混合大腦執行結果
type BrainResult struct {
	Output         string
	Executor       ExecutorType
	Tokens         int
	CostUSD        float64
	Duration       time.Duration
	Error          error
	CacheHit       bool
	Classification TaskClassification
}

// BrainStats 混合大腦統計
type BrainStats struct {
	UsedBudgetUSD   float64
	MaxBudgetUSD    float64
	RemainingBudget float64
	TotalTokens     int
	TasksExecuted   int
	BudgetWarning   bool
	// 熔斷器狀態
	CLCircuit CircuitBreakerStats
	CCCircuit CircuitBreakerStats
}

// ExecuteRequest 執行請求
type ExecuteRequest struct {
	Prompt    string
	Tools     []string
	Executor  ExecutorType // auto, cl, cc, hybrid
	Model     string
	Stream    bool
	SessionID string
}

// TaskClassification 任務分類結果
type TaskClassification struct {
	TaskType     TaskType
	Executor     ExecutorType
	Confidence   float64
	CostEstimate float64
	Reason       string
	Tools        []string
}

// TaskType 任務類型分類
type TaskType int

const (
	TaskUnknown         TaskType = iota
	TaskQuickLookup              // 快速查詢 → CL Native
	TaskFileOperation            // 檔案操作 → CL Native
	TaskDataProcessing           // 數據處理 → CL Native
	TaskGitHub                   // GitHub 任務 → Claude Code
	TaskComplexRefactor          // 複雜重構 → Claude Code
	TaskCodeReview               // 代碼審查 → Claude Code
	TaskBugHunt                  // Bug 追蹤 → Claude Code
	TaskCreative                 // 創意任務 → Claude Code
	TaskMCPTask                  // MCP 工具任務 → Claude Code
)

// DefaultHybridBrainConfig 預設配置
func DefaultHybridBrainConfig() HybridBrainConfig {
	return HybridBrainConfig{
		MaxBudgetUSD:     10.0,
		WarningThreshold: 0.8,
		MaxTurnsCL:       20,
		MaxTurnsCC:       10,
		Timeout:          5 * time.Minute,
		DefaultModel:     "sonnet",
	}
}

// HybridBrainConfig 混合大腦配置
type HybridBrainConfig struct {
	// 預算控制
	MaxBudgetUSD     float64
	WarningThreshold float64

	// 執行控制
	MaxTurnsCL int
	MaxTurnsCC int
	Timeout    time.Duration

	// 模型配置
	DefaultModel string

	// 工具配置
	AllowedToolsCL []string
	AllowedToolsCC []string
}

const maxSessionHistory = 1000

// CircuitBreakerState 熔斷器狀態
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// executorCircuitBreaker 執行者的熔斷器
type executorCircuitBreaker struct {
	mu            sync.Mutex
	state         CircuitBreakerState
	failureCount  int
	successCount  int
	lastFailure   error
	lastFailureAt time.Time
}

func newExecutorCircuitBreaker() *executorCircuitBreaker {
	return &executorCircuitBreaker{state: CircuitClosed}
}

// shouldAllow 是否允許請求
func (cb *executorCircuitBreaker) shouldAllow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// 30秒後嘗試恢復
		if time.Since(cb.lastFailureAt) > 30*time.Second {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return true
	}
}

// recordFailure 記錄失敗
func (cb *executorCircuitBreaker) recordFailure(err error) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = err
	cb.lastFailureAt = time.Now()
	cb.failureCount++

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
		return true // circuit tripped
	}

	if cb.failureCount >= 5 {
		cb.state = CircuitOpen
		return true
	}
	return false
}

// recordSuccess 記錄成功
func (cb *executorCircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.failureCount = 0

	if cb.state == CircuitHalfOpen && cb.successCount >= 2 {
		cb.state = CircuitClosed
	}
}

// getState 返回當前狀態（用於統計）
func (cb *executorCircuitBreaker) getState() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// CircuitBreakerStats 熔斷器統計
type CircuitBreakerStats struct {
	State        CircuitBreakerState
	FailureCount int
	SuccessCount int
	LastFailure  error
}

func (cb *executorCircuitBreaker) getStats() CircuitBreakerStats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return CircuitBreakerStats{
		State:        cb.state,
		FailureCount: cb.failureCount,
		SuccessCount: cb.successCount,
		LastFailure:  cb.lastFailure,
	}
}

type HybridBrainImpl struct {
	config *HybridBrainConfig

	mu             sync.Mutex // 保護以下字段的並發訪問
	usedBudgetUSD  float64
	reservedBudget float64 // 已預留但未結算的預算
	totalTokens    int
	sessionHistory []BrainTaskRecord

	// Session ID - 維護對話上下文
	sessionID string

	classifier    *TaskClassifierImpl
	costOptimizer *CostOptimizerImpl

	clClient *cl_kernel.ClKernelClient // 具體類型以支持 SetAgentRunner
	ccClient *cl_kernel.ClaudeCodeClient

	// 熔斷器
	clCircuit *executorCircuitBreaker
	ccCircuit *executorCircuitBreaker
}

// BrainTaskRecord 任務記錄
type BrainTaskRecord struct {
	Task           string
	Classification TaskClassification
	Result         BrainResult
	Timestamp      time.Time
}

// NewHybridBrain 創建新的混合大腦
func NewHybridBrain(config ...HybridBrainConfig) *HybridBrainImpl {
	cfg := DefaultHybridBrainConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	b := &HybridBrainImpl{
		config:         &cfg,
		usedBudgetUSD:  0,
		totalTokens:    0,
		sessionHistory: make([]BrainTaskRecord, 0, maxSessionHistory),
		sessionID:      uuid.New().String(), // 為每個 Brain 實例生成唯一 Session ID
		classifier:     NewTaskClassifierImpl(),
		costOptimizer:  NewCostOptimizerImpl(cfg),
		clCircuit:      newExecutorCircuitBreaker(),
		ccCircuit:      newExecutorCircuitBreaker(),
	}

	// 初始化 CL Kernel 客戶端
	clClient := cl_kernel.NewClient()
	b.clClient = clClient

	// 初始化 Claude Code 客戶端
	ccClient := cl_kernel.NewClaudeCodeClient()
	b.ccClient = ccClient

	return b
}

// NewHybridBrainWithAgent 創建帶有 Agent Runner 的混合大腦
// 這是推薦的工廠函數，可以確保真實的 MiniMax API 被調用
func NewHybridBrainWithAgent(runner cl_kernel.AgentRunner, config ...HybridBrainConfig) *HybridBrainImpl {
	b := NewHybridBrain(config...)

	// 直接使用 NewClientWithAgent 來創建已安裝 runner 的客戶端
	b.clClient = cl_kernel.NewClientWithAgent(runner)

	return b
}

// SetAgentRunner 設置 Agent Runner 以啟用真實 Agent 執行
// 這允許 HybridBrain 使用 SessionAgent 作為執行者
func (b *HybridBrainImpl) SetAgentRunner(runner cl_kernel.AgentRunner) {
	if b.clClient != nil {
		b.clClient.SetAgentRunner(runner)
	}
}

// GetCLClient 返回 CL Kernel 客戶端（用於測試）
func (b *HybridBrainImpl) GetCLClient() *cl_kernel.ClKernelClient {
	return b.clClient
}

// Think 是大腦的核心方法 - 分析並決策
func (b *HybridBrainImpl) Think(ctx context.Context, task string, forcedExecutor ...ExecutorType) *BrainResult {
	start := time.Now()

	// Step 1: 分類任務
	classification := b.classifier.Classify(task)

	// 應用強制執行者（如果指定）
	if len(forcedExecutor) > 0 && forcedExecutor[0] != "" {
		classification.Executor = forcedExecutor[0]
		classification.Reason += " [Forced]"
	}

	// Step 2: 成本評估
	costEst := b.costOptimizer.Estimate(classification)

	// Step 3: 原子預算預留與檢查
	b.mu.Lock()
	availableBudget := b.config.MaxBudgetUSD - b.usedBudgetUSD - b.reservedBudget

	if availableBudget < costEst && len(forcedExecutor) == 0 {
		// 預算不足，降級到 CL Native（CL 更便宜）
		classification.Executor = ExecutorCL
		classification.Reason += " [Budget fallback]"
		// 重新估算 CL 的成本
		costEst = b.costOptimizer.Estimate(classification)
	}

	// 如果仍有足夠預算，預留成本
	if b.config.MaxBudgetUSD-b.usedBudgetUSD-b.reservedBudget >= costEst {
		b.reservedBudget += costEst
	}
	b.mu.Unlock()

	// Step 4: 執行
	var result *BrainResult
	switch classification.Executor {
	case ExecutorCL:
		result = b.executeViaCL(ctx, task, classification)
	case ExecutorClaudeCode:
		result = b.executeViaClaudeCode(ctx, task, classification)
	default:
		result = b.executeHybrid(ctx, task, classification)
	}

	result.Duration = time.Since(start)
	result.Classification = classification

	// Step 5: 原子結算預算
	b.mu.Lock()
	// 釋放預留
	b.reservedBudget -= costEst
	// 添加實際成本
	b.usedBudgetUSD += result.CostUSD
	b.totalTokens += result.Tokens
	b.sessionHistory = append(b.sessionHistory, BrainTaskRecord{
		Task:           task,
		Classification: classification,
		Result:         *result,
		Timestamp:      time.Now(),
	})

	if len(b.sessionHistory) > maxSessionHistory {
		b.sessionHistory = b.sessionHistory[len(b.sessionHistory)-maxSessionHistory:]
	}
	b.mu.Unlock()

	return result
}

// Execute 執行任務
func (b *HybridBrainImpl) Execute(ctx context.Context, req ExecuteRequest) *BrainResult {
	executor := ExecutorType(req.Executor)
	if executor == "" {
		executor = ExecutorHybrid
	}
	return b.Think(ctx, req.Prompt, executor)
}

// GetStats 返回當前統計
func (b *HybridBrainImpl) GetStats() BrainStats {
	b.mu.Lock()
	defer b.mu.Unlock()

	remaining := b.config.MaxBudgetUSD - b.usedBudgetUSD
	if remaining < 0 {
		remaining = 0
	}
	budgetRatio := 0.0
	if b.config.MaxBudgetUSD > 0 {
		budgetRatio = b.usedBudgetUSD / b.config.MaxBudgetUSD
	}
	return BrainStats{
		UsedBudgetUSD:   b.usedBudgetUSD,
		MaxBudgetUSD:    b.config.MaxBudgetUSD,
		RemainingBudget: remaining,
		TotalTokens:     b.totalTokens,
		TasksExecuted:   len(b.sessionHistory),
		BudgetWarning:   budgetRatio > b.config.WarningThreshold,
		CLCircuit:       b.clCircuit.getStats(),
		CCCircuit:       b.ccCircuit.getStats(),
	}
}

// ClassifyTask 對任務進行分類
func (b *HybridBrainImpl) ClassifyTask(task string) TaskClassification {
	return b.classifier.Classify(task)
}

// executeViaCL 通過 CL Native 執行
func (b *HybridBrainImpl) executeViaCL(ctx context.Context, task string, c TaskClassification) *BrainResult {
	// 檢查熔斷器
	if !b.clCircuit.shouldAllow() {
		return &BrainResult{
			Executor: ExecutorCL,
			Output:   "",
			Tokens:   0,
			CostUSD:  0,
			Error:    fmt.Errorf("CL executor circuit breaker is open"),
		}
	}

	// 使用 CL Kernel Client 執行
	if b.clClient == nil {
		// 客戶端未初始化，返回錯誤
		return &BrainResult{
			Executor: ExecutorCL,
			Output:   "",
			Tokens:   0,
			CostUSD:  0,
			Error:    fmt.Errorf("CL kernel client not initialized"),
		}
	}

	req := cl_kernel.ExecuteRequest{
		Prompt:    task,
		Tools:     c.Tools,
		SessionID: b.sessionID, // 維護對話上下文
	}

	result, err := b.clClient.Execute(ctx, req)
	if err != nil {
		b.clCircuit.recordFailure(err)
		return &BrainResult{
			Executor: ExecutorCL,
			Output:   fmt.Sprintf("CL Native error: %v", err),
			Tokens:   0,
			CostUSD:  0,
			Error:    err,
		}
	}

	b.clCircuit.recordSuccess()
	return &BrainResult{
		Executor: ExecutorCL,
		Output:   result.Output,
		Tokens:   result.TotalTokens,
		CostUSD:  result.CostUSD,
		CacheHit: result.CacheHit,
		Duration: result.Duration,
	}
}

// executeViaClaudeCode 通過 Claude Code 執行
func (b *HybridBrainImpl) executeViaClaudeCode(ctx context.Context, task string, c TaskClassification) *BrainResult {
	// 檢查熔斷器
	if !b.ccCircuit.shouldAllow() {
		return &BrainResult{
			Executor: ExecutorClaudeCode,
			Output:   "",
			Tokens:   0,
			CostUSD:  0,
			Error:    fmt.Errorf("Claude Code executor circuit breaker is open"),
		}
	}

	// 使用 Claude Code Client 執行
	if b.ccClient == nil {
		// 客戶端未初始化，返回錯誤
		return &BrainResult{
			Executor: ExecutorClaudeCode,
			Output:   "",
			Tokens:   0,
			CostUSD:  0,
			Error:    fmt.Errorf("Claude Code client not initialized"),
		}
	}

	req := cl_kernel.ExecuteRequest{
		Prompt:    task,
		Tools:     c.Tools,
		SessionID: b.sessionID, // 維護對話上下文
	}

	result, err := b.ccClient.Execute(ctx, req)
	if err != nil {
		b.ccCircuit.recordFailure(err)
		return &BrainResult{
			Executor: ExecutorClaudeCode,
			Output:   fmt.Sprintf("Claude Code error: %v", err),
			Tokens:   0,
			CostUSD:  0,
			Error:    err,
		}
	}

	b.ccCircuit.recordSuccess()
	return &BrainResult{
		Executor: ExecutorClaudeCode,
		Output:   result.Output,
		Tokens:   result.TotalTokens,
		CostUSD:  result.CostUSD,
		CacheHit: result.CacheHit,
		Duration: result.Duration,
	}
}

// executeHybrid 混合執行
// 智能合併 CL 和 Claude Code 的結果
// 策略：高信心時以 CC 為主，低信心時以 CL 為主
func (b *HybridBrainImpl) executeHybrid(ctx context.Context, task string, c TaskClassification) *BrainResult {
	// 並行執行 CL 和 Claude Code 以節省時間
	clResultCh := make(chan *BrainResult, 1)
	ccResultCh := make(chan *BrainResult, 1)

	// 並行啟動兩個執行
	go func() {
		clResultCh <- b.executeViaCL(ctx, task, c)
	}()

	go func() {
		ccResultCh <- b.executeViaClaudeCode(ctx, task, c)
	}()

	// 收集結果
	var clResult, ccResult *BrainResult
	select {
	case clResult = <-clResultCh:
	case <-ctx.Done():
		return &BrainResult{
			Executor: ExecutorHybrid,
			Output:   "Task cancelled",
			Error:    ctx.Err(),
		}
	}

	select {
	case ccResult = <-ccResultCh:
	case <-ctx.Done():
		// CL 成功，CC 超時，仍返回 CL 結果
		return clResult
	case <-time.After(30 * time.Second):
		// CC 超時，返回 CL 結果
		ccResult = &BrainResult{
			Executor: ExecutorClaudeCode,
			Output:   "",
			Tokens:   0,
			CostUSD:  0,
			Error:    fmt.Errorf("Claude Code execution timeout"),
		}
	}

	// 智能合併結果
	return b.mergeResults(clResult, ccResult, c)
}

// mergeResults 智能合併 CL 和 CC 的結果
func (b *HybridBrainImpl) mergeResults(clResult, ccResult *BrainResult, c TaskClassification) *BrainResult {
	result := &BrainResult{
		Executor: ExecutorHybrid,
		Tokens:   clResult.Tokens,
		CostUSD:  clResult.CostUSD,
	}

	// 根據 Confidence 決定主要結果來源
	// Confidence > 0.7: 以 Claude Code 為主（複雜任務 CC 更擅長）
	// Confidence <= 0.7: 以 CL 為主（簡單任務 CL 更快更便宜）
	if c.Confidence > 0.7 {
		// 複雜任務：以 CC 為主
		if ccResult.Error == nil {
			result.Output = "[Hybrid - CC Primary]\n\n=== Claude Code Response ===\n" + ccResult.Output
			result.Tokens = ccResult.Tokens
			result.CostUSD = ccResult.CostUSD
			// 附加 CL 分析作為參考
			if clResult.Output != "" && clResult.Error == nil {
				result.Output += "\n\n=== CL Native Analysis ===\n" + clResult.Output
				result.Tokens += clResult.Tokens
				result.CostUSD += clResult.CostUSD
			}
		} else {
			// CC 失敗，降級到 CL
			result.Output = "[Hybrid - CL Fallback]\n" + clResult.Output
			result.Error = fmt.Errorf("CC failed: %v, fell back to CL", ccResult.Error)
		}
	} else {
		// 簡單任務：以 CL 為主
		result.Output = "[Hybrid - CL Primary]\n" + clResult.Output
		result.Tokens = clResult.Tokens
		result.CostUSD = clResult.CostUSD
		// 如果 CC 成功且有有意義的補充，附加它
		if ccResult.Error == nil && ccResult.Output != "" && len(ccResult.Output) > len(clResult.Output) {
			result.Output += "\n\n=== Claude Code Enhancement ===\n" + ccResult.Output
			result.Tokens += ccResult.Tokens
			result.CostUSD += ccResult.CostUSD
		}
	}

	// 記錄熔斷器狀態
	if ccResult.Error != nil {
		b.ccCircuit.recordFailure(ccResult.Error)
	} else {
		b.ccCircuit.recordSuccess()
	}
	if clResult.Error != nil {
		b.clCircuit.recordFailure(clResult.Error)
	} else {
		b.clCircuit.recordSuccess()
	}

	return result
}
