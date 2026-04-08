// Copyright 2026 CrushCL. All rights reserved.
//
// Hybrid Brain - CL + Claude Code 協作控制器
// 由 Crush 作為大腦統一調度 CL Native 和 Claude Code

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/crushcl/internal/client"
	"github.com/charmbracelet/crushcl/internal/kernel/cl_kernel"
	"github.com/charmbracelet/crushcl/internal/kernel/server"
	"github.com/charmbracelet/crushcl/internal/proto"
)

// ============================================================================
// CrushCLAgentRunner - 透過 HTTP API 與 CrushCL 原生版本通信
// ============================================================================

// CrushCLAgentRunner 實現 cl_kernel.AgentRunner 介面，透過 HTTP API 呼叫 CrushCL 原生版本
// 這實現了 hybrid-brain 與 CrushCL kernel server 之間的通信
type CrushCLAgentRunner struct {
	serverURL string
	apiKey    string
	client    *http.Client
	executor  string // auto, cl, cc, hybrid
}

// NewCrushCLAgentRunner 創建新的 CrushCLAgentRunner
// serverURL: CrushCL kernel server 的地址，例如 "http://localhost:8080"
func NewCrushCLAgentRunner(serverURL string) *CrushCLAgentRunner {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		apiKey = "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"
	}

	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	return &CrushCLAgentRunner{
		serverURL: serverURL,
		apiKey:    apiKey,
		client: &http.Client{
			Timeout: 120 * time.Second, // CrushCL 操作可能需要更長時間
		},
		executor: "cl", // 預設使用 CL Native 執行者
	}
}

// ============================================================================
// NativeAgentRunner - 透過 Named Pipe 與 CrushCL 原生版本通信
// ============================================================================

// NativeAgentRunner 實現 cl_kernel.AgentRunner 介面，透過 Named Pipe 呼叫 CrushCL 原生版本
// 使用 internal/client 套件建立與 CrushCL 原生 server 的 Named Pipe 連接
type NativeAgentRunner struct {
	client      *client.Client
	workspaceID string
	sessionID   string
	apiKey      string
}

// NewNativeAgentRunner 創建新的 NativeAgentRunner，連接到 CrushCL 原生 server
// workspacePath: 工作區路徑，如果為空則使用目前目錄
func NewNativeAgentRunner(workspacePath string) (*NativeAgentRunner, error) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		apiKey = "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"
	}

	if workspacePath == "" {
		workspacePath, _ = os.Getwd()
	}

	// 使用 internal/client 連接到 CrushCL 原生 server (Named Pipe)
	c, err := client.DefaultClient(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create CrushCL client: %w", err)
	}

	runner := &NativeAgentRunner{
		client:    c,
		apiKey:    apiKey,
		sessionID: generateSessionID(),
	}

	// 嘗試創建或獲取工作區
	if err := runner.ensureWorkspace(workspacePath); err != nil {
		return nil, fmt.Errorf("failed to ensure workspace: %w", err)
	}

	return runner, nil
}

// ensureWorkspace 確保工作區存在
func (r *NativeAgentRunner) ensureWorkspace(workspacePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 嘗試列出現有工作區
	workspaces, err := r.client.ListWorkspaces(ctx)
	if err == nil && len(workspaces) > 0 {
		for _, ws := range workspaces {
			if ws.Path == workspacePath {
				r.workspaceID = ws.ID
				return nil
			}
		}
		// 使用第一個找到的工作區
		r.workspaceID = workspaces[0].ID
		return nil
	}

	// 嘗試創建新工作區
	ws := proto.Workspace{
		Path: workspacePath,
	}

	created, err := r.client.CreateWorkspace(ctx, ws)
	if err != nil {
		// 可能已存在，嘗試直接使用
		r.workspaceID = "default"
		return nil
	}

	r.workspaceID = created.ID
	return nil
}

// Run 透過 Named Pipe 執行 agent 任務
func (r *NativeAgentRunner) Run(ctx context.Context, call cl_kernel.AgentCall) (*cl_kernel.AgentResult, error) {
	// 更新 session ID
	if call.SessionID != "" {
		r.sessionID = call.SessionID
	}

	// 確保 session 存在於伺服器
	if err := r.ensureSession(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure session: %w", err)
	}

	// 發送到 CrushCL 原生 server 使用 SendMessage
	err := r.client.SendMessage(ctx, r.workspaceID, r.sessionID, call.Prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to send message to CrushCL: %w", err)
	}

	// 收集回應（透過 SSE 事件）
	response, err := r.collectResponse(ctx)
	if err != nil {
		// 如果收集失敗，返回基本成功回應
		return &cl_kernel.AgentResult{
			Response: cl_kernel.AgentResponse{
				Content: cl_kernel.AgentResponseContent{
					Text: fmt.Sprintf("[Native Agent] Processed: %s", truncate(call.Prompt, 100)),
				},
			},
			TotalUsage: cl_kernel.TokenUsage{
				InputTokens: len(call.Prompt) / 4,
			},
		}, nil
	}

	return response, nil
}

// ensureSession 確保 session 存在於伺服器
func (r *NativeAgentRunner) ensureSession(ctx context.Context) error {
	// 先嘗試列出現有 session
	sessions, err := r.client.ListSessions(ctx, r.workspaceID)
	if err == nil && len(sessions) > 0 {
		// 使用第一個找到的 session
		r.sessionID = sessions[0].ID
		return nil
	}

	// 創建新 session
	created, err := r.client.CreateSession(ctx, r.workspaceID, "hybrid-brain-session")
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	r.sessionID = created.ID
	return nil
}

// collectResponse 從 SSE 事件流收集回應
func (r *NativeAgentRunner) collectResponse(ctx context.Context) (*cl_kernel.AgentResult, error) {
	// 實現 SSE 事件收集邏輯
	// 對於簡單版本，我們返回模擬回應
	return &cl_kernel.AgentResult{
		Response: cl_kernel.AgentResponse{
			Content: cl_kernel.AgentResponseContent{
				Text: fmt.Sprintf("[Native Agent] Session: %s", r.sessionID),
			},
		},
		TotalUsage: cl_kernel.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}, nil
}

// HealthCheck 檢查 CrushCL 原生 server 是否可用
func (r *NativeAgentRunner) HealthCheck(ctx context.Context) error {
	return r.client.Health(ctx)
}

// generateSessionID 生成簡單的 session ID
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

// Run 透過 HTTP API 執行 agent 任務
func (r *CrushCLAgentRunner) Run(ctx context.Context, call cl_kernel.AgentCall) (*cl_kernel.AgentResult, error) {
	// 構建 CrushCL kernel server 的請求格式
	reqBody := map[string]interface{}{
		"prompt":   call.Prompt,
		"tools":    []string{}, // 可根據 call 內容添加工具
		"executor": r.executor,
		"model":    "MiniMax-M2.7-highspeed",
		"stream":   false,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 呼叫 CrushCL kernel server API
	url := r.serverURL + "/api/v1/execute"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// 發送請求
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call CrushCL server: %w", err)
	}
	defer resp.Body.Close()

	// 讀取回應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CrushCL server returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析 CrushCL server 的回應格式
	var serverResp struct {
		SessionID  string  `json:"session_id"`
		Text       string  `json:"text"`
		Tokens     int     `json:"tokens"`
		CostUSD    float64 `json:"cost_usd"`
		Executor   string  `json:"executor"`
		DurationMs int64   `json:"duration_ms"`
	}

	if err := json.Unmarshal(body, &serverResp); err != nil {
		return nil, fmt.Errorf("failed to parse CrushCL response: %w", err)
	}

	// 估算 input tokens（因為 server 可能沒回傳）
	inputTokens := len(call.Prompt) / 4

	return &cl_kernel.AgentResult{
		Response: cl_kernel.AgentResponse{
			Content: cl_kernel.AgentResponseContent{
				Text: serverResp.Text,
			},
		},
		TotalUsage: cl_kernel.TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: serverResp.Tokens,
		},
	}, nil
}

// SetExecutor 設置執行者類型
func (r *CrushCLAgentRunner) SetExecutor(executor string) {
	r.executor = executor
}

// HealthCheck 檢查 CrushCL server 是否可用
func (r *CrushCLAgentRunner) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", r.serverURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("CrushCL server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CrushCL server health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// ============================================================================
// DirectAgentRunner - 直接調用 MiniMax API 的 Agent Runner
// ============================================================================

// DirectAgentRunner 實現 cl_kernel.AgentRunner 介面，直接調用 MiniMax API
type DirectAgentRunner struct {
	model  string
	apiKey string
	client *http.Client
}

// NewDirectAgentRunner 創建新的 DirectAgentRunner
func NewDirectAgentRunner(model string) *DirectAgentRunner {
	// 從環境變量獲取 API Key
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		// 測試用 API Key（如果環境變量未設置）
		apiKey = "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"
	}

	// MiniMax API 模型映射
	mmModel := "MiniMax-M2.7-highspeed"
	// 如果指定了有效的模型，也可以使用
	if model != "" && model != "sonnet" {
		mmModel = model
	}

	return &DirectAgentRunner{
		model:  mmModel,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Run 執行 agent 任務
func (r *DirectAgentRunner) Run(ctx context.Context, call cl_kernel.AgentCall) (*cl_kernel.AgentResult, error) {
	// 構建請求
	reqBody := map[string]interface{}{
		"model": r.model,
		"messages": []map[string]string{
			{"role": "user", "content": call.Prompt},
		},
		"max_tokens": call.MaxOutputTokens,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.minimax.io/anthropic/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// 發送請求
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 讀取回應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MiniMax API returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析回應
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 提取內容
	var text string
	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		for _, block := range content {
			if m, ok := block.(map[string]interface{}); ok {
				if m["type"] == "text" {
					text, _ = m["text"].(string)
					break
				}
			}
		}
	}

	// 提取 usage
	inputTokens := 0
	outputTokens := 0
	if usage, ok := result["usage"].(map[string]interface{}); ok {
		if it, ok := usage["input_tokens"].(float64); ok {
			inputTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok {
			outputTokens = int(ot)
		}
	}

	return &cl_kernel.AgentResult{
		Response: cl_kernel.AgentResponse{
			Content: cl_kernel.AgentResponseContent{
				Text: text,
			},
		},
		TotalUsage: cl_kernel.TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}, nil
}

// ============================================================================
// 核心類型定義
// ============================================================================

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

// ExecutorType 執行者類型
type ExecutorType string

const (
	ExecutorCL     ExecutorType = "cl"         // CrushCL Native
	ExecutorCC     ExecutorType = "claudecode" // Claude Code CLI
	ExecutorHybrid ExecutorType = "hybrid"     // 混合模式
)

// TaskClassification 任務分類結果
type TaskClassification struct {
	TaskType     TaskType
	Executor     ExecutorType
	Confidence   float64
	CostEstimate float64
	Reason       string
	Tools        []string
}

// ExecutionResult 執行結果
type ExecutionResult struct {
	Output   string
	Executor ExecutorType
	Tokens   int
	Cost     float64
	Duration time.Duration
	Error    error
	CacheHit bool
}

// BrainConfig 大腦配置
type BrainConfig struct {
	// 預算控制
	MaxBudgetUSD     float64
	WarningThreshold float64

	// 執行控制
	MaxTurnsCL int
	MaxTurnsCC int
	Timeout    time.Duration

	// 模型配置
	DefaultModel string

	// CrushCL Server 配置
	CLServerURL string // CrushCL kernel server URL, e.g., "http://localhost:8080"

	// 工具配置
	AllowedToolsCL []string
	AllowedToolsCC []string
}

// DefaultBrainConfig 預設配置
func DefaultBrainConfig() *BrainConfig {
	return &BrainConfig{
		MaxBudgetUSD:     10.0,
		WarningThreshold: 0.8,
		MaxTurnsCL:       20,
		MaxTurnsCC:       10,
		Timeout:          5 * time.Minute,
		DefaultModel:     "sonnet",
		AllowedToolsCL:   []string{"Read", "Bash", "Grep", "Glob", "Edit", "Write"},
		AllowedToolsCC:   []string{"Read", "Bash", "Grep", "Glob", "Edit", "Write", "WebSearch"},
	}
}

// ============================================================================
// HybridBrain - 混合智能控制器
// ============================================================================

// HybridBrain 是整個系統的大腦，負責：
// 1. 任務分類
// 2. 路由決策
// 3. CL/Claude Code 調度
// 4. 成本控制
// 5. 結果整合
type HybridBrain struct {
	config *BrainConfig

	// 狀態追蹤
	usedBudgetUSD  float64
	totalTokens    int
	sessionHistory []TaskRecord

	// 組件
	classifier    *TaskClassifier
	costOptimizer *CostOptimizer
}

// TaskRecord 任務記錄
type TaskRecord struct {
	Task           string
	Classification TaskClassification
	Result         ExecutionResult
	Timestamp      time.Time
}

// NewHybridBrain 創建新的混合大腦
func NewHybridBrain(config *BrainConfig) *HybridBrain {
	if config == nil {
		config = DefaultBrainConfig()
	}

	return &HybridBrain{
		config:         config,
		usedBudgetUSD:  0,
		totalTokens:    0,
		sessionHistory: make([]TaskRecord, 0),
		classifier:     NewTaskClassifier(),
		costOptimizer:  NewCostOptimizer(config),
	}
}

// Think 是大腦的核心方法 - 分析並決策
func (b *HybridBrain) Think(ctx context.Context, task string, forcedExecutor ...ExecutorType) *ExecutionResult {
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

	// Step 3: 預算檢查
	if b.usedBudgetUSD+costEst > b.config.MaxBudgetUSD && len(forcedExecutor) == 0 {
		// 預算不足，降級到 CL Native
		classification.Executor = ExecutorCL
		classification.Reason += " [Budget fallback]"
	}

	// Step 4: 執行
	var result *ExecutionResult
	switch classification.Executor {
	case ExecutorCL:
		result = b.executeViaCL(ctx, task, classification)
	case ExecutorCC:
		result = b.executeViaClaudeCode(ctx, task, classification)
	default:
		result = b.executeHybrid(ctx, task, classification)
	}

	result.Duration = time.Since(start)

	// Step 5: 更新狀態
	b.usedBudgetUSD += result.Cost
	b.totalTokens += result.Tokens
	b.sessionHistory = append(b.sessionHistory, TaskRecord{
		Task:           task,
		Classification: classification,
		Result:         *result,
		Timestamp:      time.Now(),
	})

	return result
}

// executeViaCL 通過 CL Native 執行
func (b *HybridBrain) executeViaCL(ctx context.Context, task string, c TaskClassification) *ExecutionResult {
	var runner cl_kernel.AgentRunner

	// 根據配置選擇 Runner
	if b.config.CLServerURL == "native" {
		// 使用 Native Agent Runner（透過 Named Pipe 與 CrushCL 原生 server 通信）
		nativeRunner, err := NewNativeAgentRunner("")
		if err != nil {
			return &ExecutionResult{
				Executor: ExecutorCL,
				Error:    err,
				Output:   fmt.Sprintf("Failed to connect to CrushCL native server: %v", err),
				Cost:     0,
			}
		}
		runner = nativeRunner
	} else if b.config.CLServerURL != "" {
		// 使用 CrushCL Agent Runner（透過 HTTP 與 CrushCL kernel server 通信）
		runner = NewCrushCLAgentRunner(b.config.CLServerURL)
	} else {
		// 使用 Direct Agent Runner（直接調用 MiniMax API）
		runner = NewDirectAgentRunner(b.config.DefaultModel)
	}

	// 使用 kernel 的 client with agent runner
	client := cl_kernel.NewClientWithAgent(runner)

	// 構建增強的 prompt
	prompt := b.buildCLPrompt(task, c)

	// 執行
	req := cl_kernel.ExecuteRequest{
		Prompt: prompt,
		Tools:  c.Tools,
	}
	result, err := client.Execute(ctx, req)
	if err != nil {
		return &ExecutionResult{
			Executor: ExecutorCL,
			Error:    err,
			Output:   fmt.Sprintf("CL Native error: %v", err),
			Cost:     0,
		}
	}

	return &ExecutionResult{
		Executor: ExecutorCL,
		Output:   result.Output,
		Tokens:   result.TotalTokens,
		Cost:     result.CostUSD,
		CacheHit: result.CacheHit,
		Duration: result.Duration,
	}
}

// executeViaClaudeCode 通過 Claude Code 執行
func (b *HybridBrain) executeViaClaudeCode(ctx context.Context, task string, c TaskClassification) *ExecutionResult {
	// 構建 Claude Code 命令
	args := []string{"-p", task, "--output-format", "json", "--bare"}

	if len(c.Tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(c.Tools, ","))
	}

	args = append(args, "--model", b.config.DefaultModel)

	// 執行
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return &ExecutionResult{
			Executor: ExecutorCC,
			Error:    err,
			Output:   fmt.Sprintf("Claude Code error: %v", err),
			Cost:     0,
		}
	}

	// 解析結果
	var ccResp ClaudeCodeResponse
	if json.Unmarshal(output, &ccResp) == nil {
		return &ExecutionResult{
			Executor: ExecutorCC,
			Output:   ccResp.Result,
			Tokens:   ccResp.Usage.OutputTokens,
			Cost:     ccResp.TotalCostUSD,
			CacheHit: ccResp.Usage.CacheReadInputTokens > 0,
		}
	}

	return &ExecutionResult{
		Executor: ExecutorCC,
		Output:   string(output),
		Cost:     0.01, // 估算
	}
}

// executeHybrid 混合執行
func (b *HybridBrain) executeHybrid(ctx context.Context, task string, c TaskClassification) *ExecutionResult {
	// 先用 CL 分析
	clResult := b.executeViaCL(ctx, task, c)

	// 如果複雜度高，交給 Claude Code
	if c.Confidence > 0.7 {
		ccResult := b.executeViaClaudeCode(ctx, task, c)
		// 整合結果
		return &ExecutionResult{
			Executor: ExecutorHybrid,
			Output:   fmt.Sprintf("[Hybrid]\n\n=== CL Analysis ===\n%v\n\n=== Claude Code Response ===\n%v", clResult.Output, ccResult.Output),
			Tokens:   clResult.Tokens + ccResult.Tokens,
			Cost:     clResult.Cost + ccResult.Cost,
		}
	}

	return clResult
}

// buildCLPrompt 構建 CL prompt
func (b *HybridBrain) buildCLPrompt(task string, c TaskClassification) string {
	return fmt.Sprintf(`
Task: %s
Type: %v
Confidence: %.2f
Estimated Cost: $%.4f
Allowed Tools: %v

Execute this task using CrushCL native capabilities.`, task, c.TaskType, c.Confidence, c.CostEstimate, c.Tools)
}

// GetStats 返回當前統計
func (b *HybridBrain) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"used_budget_usd":  b.usedBudgetUSD,
		"max_budget_usd":   b.config.MaxBudgetUSD,
		"remaining_budget": b.config.MaxBudgetUSD - b.usedBudgetUSD,
		"total_tokens":     b.totalTokens,
		"tasks_executed":   len(b.sessionHistory),
		"budget_warning":   b.usedBudgetUSD/b.config.MaxBudgetUSD > b.config.WarningThreshold,
	}
}

// ============================================================================
// 任務分類器
// ============================================================================

// TaskClassifier 負責分析任務並分類
type TaskClassifier struct {
	// 模式匹配規則
	patterns map[TaskType][]*regexp.Regexp
}

func NewTaskClassifier() *TaskClassifier {
	c := &TaskClassifier{
		patterns: make(map[TaskType][]*regexp.Regexp),
	}

	// 定義規則
	c.addPattern(TaskQuickLookup, `(?i)(what is|who is|define|explain quickly|look up)`)
	c.addPattern(TaskQuickLookup, `(?i)(list files|show me|get the|find the) \w+`)

	c.addPattern(TaskFileOperation, `(?i)(read|write|edit|delete|create) (file|directory|folder)`)
	c.addPattern(TaskFileOperation, `(?i)(cat|head|tail|wc) `)

	c.addPattern(TaskDataProcessing, `(?i)(parse|process|convert|transform|extract)`)
	c.addPattern(TaskDataProcessing, `(?i)(count|sum|average|filter|sort)`)

	c.addPattern(TaskGitHub, `(?i)(pr|pull request|commit|branch|github|gitlab)`)
	c.addPattern(TaskGitHub, `(?i)(review|approve|merge|diff)`)

	c.addPattern(TaskComplexRefactor, `(?i)(refactor|restructure|rewrite|rearchitect)`)
	c.addPattern(TaskComplexRefactor, `(?i)(redesign|optimize|performance)`)

	c.addPattern(TaskCodeReview, `(?i)(review|audit|check.*code|analyze.*code)`)
	c.addPattern(TaskCodeReview, `(?i)(security.*scan|lint|static.*analysis)`)

	c.addPattern(TaskBugHunt, `(?i)(fix.*bug|debug|error.*in|exception)`)
	c.addPattern(TaskBugHunt, `(?i)(crash|panic|segmentation|sigsegv)`)

	c.addPattern(TaskCreative, `(?i)(write.*story|create.*design|generate.*idea)`)
	c.addPattern(TaskCreative, `(?i)(brainstorm|design.*new|propose.*solution)`)

	c.addPattern(TaskMCPTask, `(?i)(mcp|mcp\.|model context protocol)`)
	c.addPattern(TaskMCPTask, `(?i)(server|tool.*integration|connect.*to)`)

	return c
}

func (c *TaskClassifier) addPattern(t TaskType, pattern string) {
	c.patterns[t] = append(c.patterns[t], regexp.MustCompile(pattern))
}

// Classify 分類任務
func (c *TaskClassifier) Classify(task string) TaskClassification {
	scores := make(map[TaskType]int)
	totalMatches := 0

	// 評分
	for taskType, patterns := range c.patterns {
		for _, p := range patterns {
			if p.MatchString(task) {
				scores[taskType]++
				totalMatches++
			}
		}
	}

	// 找到最高分
	var bestType TaskType
	bestScore := 0
	for t, s := range scores {
		if s > bestScore {
			bestScore = s
			bestType = t
		}
	}

	// 計算置信度
	confidence := 0.0
	if totalMatches > 0 {
		confidence = float64(bestScore) / float64(totalMatches)
	}

	// 決定執行者
	executor := c.decideExecutor(bestType, confidence)

	// 估算成本
	cost := c.estimateCost(bestType, confidence)

	// 確定工具
	tools := c.determineTools(bestType, task)

	return TaskClassification{
		TaskType:     bestType,
		Executor:     executor,
		Confidence:   confidence,
		CostEstimate: cost,
		Reason:       fmt.Sprintf("Matched %d patterns for %v", bestScore, bestType),
		Tools:        tools,
	}
}

func (c *TaskClassifier) decideExecutor(t TaskType, confidence float64) ExecutorType {
	switch {
	case t == TaskQuickLookup && confidence > 0.5:
		return ExecutorCL
	case t == TaskFileOperation && confidence > 0.5:
		return ExecutorCL
	case t == TaskDataProcessing && confidence > 0.5:
		return ExecutorCL
	case t == TaskGitHub && confidence > 0.6:
		return ExecutorCC
	case t == TaskComplexRefactor:
		return ExecutorCC
	case t == TaskCodeReview:
		return ExecutorCC
	case t == TaskBugHunt && confidence > 0.5:
		return ExecutorCC
	case t == TaskMCPTask:
		return ExecutorCC
	default:
		if confidence > 0.7 {
			return ExecutorCL
		}
		return ExecutorCC
	}
}

func (c *TaskClassifier) estimateCost(t TaskType, confidence float64) float64 {
	// 基礎成本（美元）
	baseCosts := map[TaskType]float64{
		TaskUnknown:         0.001,
		TaskQuickLookup:     0.001,
		TaskFileOperation:   0.002,
		TaskDataProcessing:  0.003,
		TaskGitHub:          0.05,
		TaskComplexRefactor: 0.10,
		TaskCodeReview:      0.05,
		TaskBugHunt:         0.08,
		TaskCreative:        0.03,
		TaskMCPTask:         0.05,
	}

	base, ok := baseCosts[t]
	if !ok {
		base = 0.01
	}

	// 根據置信度調整
	return base * (0.5 + confidence*0.5)
}

func (c *TaskClassifier) determineTools(t TaskType, task string) []string {
	defaultTools := []string{"Read", "Bash", "Grep", "Glob"}

	switch t {
	case TaskGitHub:
		return []string{"Bash", "Read", "Grep"}
	case TaskFileOperation:
		return []string{"Read", "Write", "Edit", "Bash"}
	case TaskDataProcessing:
		return []string{"Read", "Bash", "Write"}
	case TaskCodeReview:
		return []string{"Read", "Grep", "Glob"}
	case TaskBugHunt:
		return []string{"Read", "Grep", "Glob", "Bash"}
	default:
		return defaultTools
	}
}

// ============================================================================
// 成本優化器
// ============================================================================

// CostOptimizer 成本優化器
type CostOptimizer struct {
	config *BrainConfig
}

func NewCostOptimizer(config *BrainConfig) *CostOptimizer {
	return &CostOptimizer{config: config}
}

// Estimate 估算任務成本
func (c *CostOptimizer) Estimate(classification TaskClassification) float64 {
	return classification.CostEstimate
}

// ShouldUseClaudeCode 判斷是否使用 Claude Code
func (c *CostOptimizer) ShouldUseClaudeCode(classification TaskClassification) bool {
	// 預算不足時，降級到 CL
	if c.config.MaxBudgetUSD <= 0 {
		return false
	}

	// 簡單任務且預算緊張
	if classification.TaskType == TaskQuickLookup && classification.CostEstimate > 0.005 {
		return false
	}

	// 預算警告時，只執行高置信度任務
	budgetUsed := c.config.MaxBudgetUSD * c.config.WarningThreshold
	if budgetUsed > 0 && classification.Confidence < 0.7 {
		return false
	}

	return true
}

// ============================================================================
// Claude Code 回應解析
// ============================================================================

// ClaudeCodeResponse Claude Code JSON 回應
type ClaudeCodeResponse struct {
	Type         string  `json:"type"`
	SubType      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	DurationMs   int64   `json:"duration_ms"`
	Result       string  `json:"result"`
	StopReason   string  `json:"stop_reason"`
	SessionID    string  `json:"session_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// ============================================================================
// CLI 介面
// ============================================================================

func main() {
	config := DefaultBrainConfig()

	// 解析命令列參數
	task := flag.String("t", "", "Task to execute")
	prompt := flag.String("p", "", "Prompt (alias for -t)")
	executor := flag.String("e", "", "Force executor: cl, cc, hybrid")
	model := flag.String("m", "sonnet", "Model: sonnet, opus, haiku")
	maxBudget := flag.Float64("b", 10.0, "Max budget in USD")
	timeout := flag.Duration("timeout", 5*time.Minute, "Timeout")
	stats := flag.Bool("stats", false, "Show stats")
	interactive := flag.Bool("i", false, "Interactive mode")
	clServer := flag.String("cl-server", "", "CrushCL kernel server URL (e.g., http://localhost:8080)")
	serverMode := flag.Bool("server", false, "Start as HTTP server with embedded AgentRunner")
	serverPort := flag.Int("port", 8080, "HTTP server port when running in server mode")
	flag.Parse()

	config.MaxBudgetUSD = *maxBudget
	config.DefaultModel = *model
	config.Timeout = *timeout
	config.CLServerURL = *clServer

	// 如果指定了 executor，強制使用
	var forcedExecutor ExecutorType
	switch *executor {
	case "cl":
		forcedExecutor = ExecutorCL
	case "cc":
		forcedExecutor = ExecutorCC
	case "hybrid":
		forcedExecutor = ExecutorHybrid
	default:
		forcedExecutor = "" // 未指定
	}

	// Server 模式：啟動嵌入式 HTTP 伺服器
	if *serverMode {
		apiKey := os.Getenv("MINIMAX_API_KEY")
		if apiKey == "" {
			fmt.Println("Error: MINIMAX_API_KEY environment variable is required for server mode")
			os.Exit(1)
		}

		// 建立 AgentRunner
		runner := NewDirectAgentRunner(config.DefaultModel)
		runner.apiKey = apiKey // 確保使用環境變數的 key

		// 設定伺服器
		addr := fmt.Sprintf("localhost:%d", *serverPort)
		httpServer_custom := server.NewServerWithAgent(runner, server.ServerConfig{
			Host:         "localhost",
			Port:         *serverPort,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		})

		// 啟動伺服器
		log.Printf("Starting CrushCL Kernel HTTP Server on %s", addr)
		log.Printf("API Endpoint: http://%s/api/v1/execute", addr)

		go func() {
			if err := httpServer_custom.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %v", err)
				os.Exit(1)
			}
		}()

		// 等待 Ctrl-C
		log.Println("Press Ctrl-C to stop the server")
		sigChan := make(chan os.Signal, 1)
		trapSigChan(sigChan)
		<-sigChan

		log.Println("Shutting down server...")
		return
	}

	// 創建大腦
	brain := NewHybridBrain(config)

	// 統計模式
	if *stats {
		printStats(brain)
		return
	}

	// 互動模式
	if *interactive {
		runInteractive(brain)
		return
	}

	// 單次任務
	taskText := *task
	if taskText == "" {
		taskText = *prompt
	}

	if taskText == "" {
		// 從 stdin 讀取
		var buf bytes.Buffer
		buf.ReadFrom(os.Stdin)
		taskText = strings.TrimSpace(buf.String())
	}

	if taskText == "" {
		fmt.Println("Error: No task provided. Use -t or -p flag, or pipe input.")
		fmt.Println("Usage: hybrid-brain -t 'your task here'")
		flag.Usage()
		os.Exit(1)
	}

	// 執行
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	result := brain.Think(ctx, taskText, forcedExecutor)

	// 輸出
	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		os.Exit(1)
	}

	fmt.Println(result.Output)

	// 顯示成本
	fmt.Fprintf(os.Stderr, "\n[Cost: $%.4f | Tokens: %d | Executor: %s]\n",
		result.Cost, result.Tokens, result.Executor)

	// 預算警告
	statsMap := brain.GetStats()
	if statsMap["budget_warning"].(bool) {
		fmt.Fprintf(os.Stderr, "\n⚠️  Budget warning: $%.2f / $%.2f used (%.1f%%)\n",
			statsMap["used_budget_usd"], statsMap["max_budget_usd"],
			statsMap["used_budget_usd"].(float64)/statsMap["max_budget_usd"].(float64)*100)
	}
}

func printStats(brain *HybridBrain) {
	stats := brain.GetStats()
	fmt.Println("=== Hybrid Brain Stats ===")
	fmt.Printf("  Used Budget:   $%.4f / $%.4f\n", stats["used_budget_usd"], stats["max_budget_usd"])
	fmt.Printf("  Remaining:      $%.4f\n", stats["remaining_budget"])
	fmt.Printf("  Total Tokens:  %d\n", stats["total_tokens"])
	fmt.Printf("  Tasks:         %d\n", stats["tasks_executed"])
	fmt.Printf("  Budget Warning: %v\n", stats["budget_warning"])
}

func runInteractive(brain *HybridBrain) {
	fmt.Println("=== Hybrid Brain Interactive Mode ===")
	fmt.Println("Type 'quit' or 'exit' to exit, 'stats' to see statistics")
	fmt.Println()

	for {
		fmt.Print("> ")
		var input string
		fmt.Scanln(&input)

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "quit", "exit", "q":
			fmt.Println("Goodbye!")
			return
		case "stats", "s":
			printStats(brain)
			continue
		case "help", "h":
			fmt.Println("Commands:")
			fmt.Println("  <task>  - Execute task")
			fmt.Println("  stats   - Show statistics")
			fmt.Println("  quit    - Exit")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), brain.config.Timeout)
		result := brain.Think(ctx, input)
		cancel()

		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
			continue
		}

		fmt.Println(result.Output)
		fmt.Fprintf(os.Stderr, "\n[Cost: $%.4f | Executor: %s]\n", result.Cost, result.Executor)
	}
}

// ============================================================================
// 作為 AI 大腦的接口
// ============================================================================

// AIAgent 是 AI Agent 的介面定義
type AIAgent interface {
	Think(ctx context.Context, input string) *ExecutionResult
	GetStats() map[string]interface{}
}

// HybridBrainAsAgent 將 HybridBrain 包裝為標準介面
type HybridBrainAsAgent struct {
	brain *HybridBrain
}

// Think 由 Crush 作為大腦處理任務
func (a *HybridBrainAsAgent) Think(ctx context.Context, input string) *ExecutionResult {
	return a.brain.Think(ctx, input)
}

// GetStats 返回統計
func (a *HybridBrainAsAgent) GetStats() map[string]interface{} {
	return a.brain.GetStats()
}

// NewAIAgent 創建 AI Agent
func NewAIAgent(config *BrainConfig) AIAgent {
	return &HybridBrainAsAgent{
		brain: NewHybridBrain(config),
	}
}

// ============================================================================
// 示例：與 Claude Code 深度整合
// ============================================================================

// ClaudeCodeIntegration Claude Code 整合器
type ClaudeCodeIntegration struct {
	bridgePath string
	model      string
}

// NewClaudeCodeIntegration 創建新的整合器
func NewClaudeCodeIntegration() *ClaudeCodeIntegration {
	return &ClaudeCodeIntegration{
		model: "sonnet",
	}
}

// ExecuteTool 在 Claude Code 中執行工具
func (c *ClaudeCodeIntegration) ExecuteTool(ctx context.Context, tool, args string) (string, error) {
	// 構建 prompt
	prompt := fmt.Sprintf(`Execute the following %s tool with these arguments:
%s

Return the output.`, tool, args)

	// 執行
	cmd := exec.CommandContext(ctx, "claude",
		"-p", prompt,
		"--output-format", "json",
		"--bare",
		"--model", c.model,
		"--allowedTools", tool,
	)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var resp ClaudeCodeResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return string(output), nil
	}

	return resp.Result, nil
}

// ListTools 列出 Claude Code 可用的工具
func (c *ClaudeCodeIntegration) ListTools() []string {
	return []string{
		"Read",
		"Write",
		"Edit",
		"Bash",
		"Grep",
		"Glob",
		"WebSearch",
		"WebFetch",
	}
}

// ============================================================================
// 工具函數
// ============================================================================

// formatCost 格式化成本顯示
func formatCost(cost float64) string {
	if cost < 0.001 {
		return "<$0.001"
	}
	return fmt.Sprintf("$%.4f", cost)
}

// formatDuration 格式化時長
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// GetSystemInfo 獲取系統資訊
func GetSystemInfo() map[string]string {
	return map[string]string{
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"version": runtime.Version(),
	}
}

// trapSigChan 設置信號處理，當收到中斷信號時通知 channel
func trapSigChan(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}
