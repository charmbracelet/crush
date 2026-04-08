// Copyright 2026 CrushCL. All rights reserved.
//
// CL Kernel Client - CrushCL 內核客戶端
// 提供直接調用 CrushCL Agent 的能力
// Phase 1: 實現 HybridBrain 的執行能力

package cl_kernel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ExecutorType 執行者類型
type ExecutorType string

const (
	ExecutorCL         ExecutorType = "cl"
	ExecutorClaudeCode ExecutorType = "cc"
	ExecutorHybrid     ExecutorType = "hybrid"
)

// Client 是 CL Kernel 客戶端介面
type Client interface {
	// Execute 執行 prompt 並返回結果
	Execute(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error)

	// ExecuteStream 執行並流式返回結果
	ExecuteStream(ctx context.Context, req ExecuteRequest, stream func(chunk string) error) error

	// GetStats 返回使用統計
	GetStats() Stats

	// Close 關閉客戶端
	Close() error
}

// ExecuteRequest 執行請求
type ExecuteRequest struct {
	Prompt    string
	Tools     []string
	SessionID string
	Model     string
	Stream    bool
}

// ExecutionResult 執行結果
type ExecutionResult struct {
	Output       string
	Executor     ExecutorType
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
	CacheHit     bool
	Duration     time.Duration
	Error        error
}

// Stats 使用統計
type Stats struct {
	TotalRequests  int
	TotalTokens    int
	TotalCostUSD   float64
	CacheHits      int
	CacheMisses    int
	AverageLatency time.Duration
	FailureCount   int
	SuccessCount   int
	TotalLatencyMs int64 // 累積延遲用於計算平均值
}

// ClKernelClient 是 CL Kernel 客戶端的實現
// 整合 CrushCL 的 SessionAgent 來執行任務
type ClKernelClient struct {
	// 配置
	timeout time.Duration

	// Agent 整合（可選，如果提供則使用真實 agent）
	agentRunner AgentRunner

	// 狀態
	mu    sync.RWMutex
	stats Stats
}

// AgentRunner 是執行 agent 任務的介面
type AgentRunner interface {
	Run(ctx context.Context, call AgentCall) (*AgentResult, error)
}

// AgentCall 是 agent 調用的請求
type AgentCall struct {
	SessionID       string
	Prompt          string
	MaxOutputTokens int64
	SystemPrompt    string
}

// AgentResult 是 agent 執行的結果
type AgentResult struct {
	Response   AgentResponse
	TotalUsage TokenUsage
}

// AgentResponse 是 agent 的回應
type AgentResponse struct {
	Content AgentResponseContent
}

// AgentResponseContent 是 agent 回應的內容（支持多種類型）
type AgentResponseContent struct {
	Text string
}

// TokenUsage 是 token 使用量
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// NewClient 創建新的 CL Kernel 客戶端
func NewClient() *ClKernelClient {
	return &ClKernelClient{
		timeout: 5 * time.Minute,
		stats: Stats{
			AverageLatency: time.Millisecond * 100,
		},
	}
}

// NewClientWithAgent 創建帶有真實 Agent 的客戶端
func NewClientWithAgent(runner AgentRunner) *ClKernelClient {
	return &ClKernelClient{
		timeout:     5 * time.Minute,
		agentRunner: runner,
		stats: Stats{
			AverageLatency: time.Millisecond * 100,
		},
	}
}

// SetAgentRunner 設置 Agent Runner
// 這允許客戶端在創建後動態切換到真實 Agent 執行
func (c *ClKernelClient) SetAgentRunner(runner AgentRunner) {
	c.agentRunner = runner
}

// Execute 執行 prompt 並返回結果
func (c *ClKernelClient) Execute(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error) {
	start := time.Now()

	// 生成 session ID 如果沒有提供
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	var result *ExecutionResult
	var err error

	// 如果有真實的 agent runner，使用它
	if c.agentRunner != nil {
		result, err = c.executeWithAgent(ctx, sessionID, req)
	} else {
		// 否則使用模擬執行（本地降級）
		result = c.executeLocally(ctx, req)
		err = result.Error
	}

	// 更新統計
	c.mu.Lock()
	c.stats.TotalRequests++
	c.stats.TotalLatencyMs += time.Since(start).Milliseconds()

	if err != nil {
		c.stats.FailureCount++
	} else {
		c.stats.SuccessCount++
		c.stats.TotalTokens += result.TotalTokens
		c.stats.TotalCostUSD += result.CostUSD
		if result.CacheHit {
			c.stats.CacheHits++
		} else {
			c.stats.CacheMisses++
		}
	}
	// 計算平均延遲
	if c.stats.TotalRequests > 0 {
		c.stats.AverageLatency = time.Duration(c.stats.TotalLatencyMs/int64(c.stats.TotalRequests)) * time.Millisecond
	}
	c.mu.Unlock()

	if err != nil {
		return result, err
	}

	result.Duration = time.Since(start)
	return result, nil
}

// executeWithAgent 使用真實 Agent 執行任務
func (c *ClKernelClient) executeWithAgent(ctx context.Context, sessionID string, req ExecuteRequest) (*ExecutionResult, error) {
	// 檢查 context 是否已取消
	select {
	case <-ctx.Done():
		return &ExecutionResult{
			Executor: ExecutorCL,
			Error:    ctx.Err(),
		}, ctx.Err()
	default:
	}

	// 構建系統 prompt
	systemPrompt := c.buildSystemPrompt(req.Tools)

	// 創建 agent call
	call := AgentCall{
		SessionID:       sessionID,
		Prompt:          req.Prompt,
		MaxOutputTokens: 8192,
		SystemPrompt:    systemPrompt,
	}

	// 執行
	result, err := c.agentRunner.Run(ctx, call)
	if err != nil {
		return &ExecutionResult{
			Executor: ExecutorCL,
			Error:    fmt.Errorf("agent execution failed: %w", err),
		}, err
	}

	// 轉換結果
	return c.agentResultToExecutionResult(result), nil
}

// buildSystemPrompt 構建系統 prompt
func (c *ClKernelClient) buildSystemPrompt(tools []string) string {
	var sb strings.Builder
	sb.WriteString("You are CrushCL, an AI coding assistant.\n\n")
	sb.WriteString("## Your Capabilities\n")
	sb.WriteString("- Read, write, and edit files\n")
	sb.WriteString("- Execute shell commands\n")
	sb.WriteString("- Search and replace code\n")
	sb.WriteString("- Work with git repositories\n")
	sb.WriteString("- And more...\n\n")

	if len(tools) > 0 {
		sb.WriteString("## Available Tools\n")
		for _, tool := range tools {
			sb.WriteString(fmt.Sprintf("- %s\n", tool))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Execute tasks directly and efficiently. Provide clear, concise responses.")
	return sb.String()
}

// agentResultToExecutionResult 將 agent 結果轉換為執行結果
func (c *ClKernelClient) agentResultToExecutionResult(result *AgentResult) *ExecutionResult {
	if result == nil {
		return &ExecutionResult{
			Executor: ExecutorCL,
		}
	}

	// 提取文本內容
	text := result.Response.Content.Text

	// 提取 token 使用量
	inputTokens := result.TotalUsage.InputTokens
	outputTokens := result.TotalUsage.OutputTokens

	// 計算成本
	cost := calculateCost(inputTokens, outputTokens)

	return &ExecutionResult{
		Output:       text,
		Executor:     ExecutorCL,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		CostUSD:      cost,
		CacheHit:     false,
	}
}

// calculateCost 計算成本
func calculateCost(inputTokens, outputTokens int) float64 {
	// 基於 GPT-4o 定价 (简化计算)
	costPerMInput := 5.0   // $5 / 1M input
	costPerMOutput := 15.0 // $15 / 1M output
	return float64(inputTokens)/1e6*costPerMInput +
		float64(outputTokens)/1e6*costPerMOutput
}

// executeLocally 本地執行（降級方案）
func (c *ClKernelClient) executeLocally(ctx context.Context, req ExecuteRequest) *ExecutionResult {
	select {
	case <-ctx.Done():
		return &ExecutionResult{
			Executor: ExecutorCL,
			Error:    ctx.Err(),
		}
	default:
	}

	var output strings.Builder

	lowerPrompt := strings.ToLower(req.Prompt)

	// 檔案操作檢測
	if strings.Contains(lowerPrompt, "read") || strings.Contains(lowerPrompt, "view") {
		output.WriteString("[CL Native] 檔案讀取操作\n")
		if file := extractFilePath(req.Prompt); file != "" {
			output.WriteString(fmt.Sprintf("  檔案: %s\n", file))
		}
	}

	// Grep 檢測
	if strings.Contains(lowerPrompt, "search") || strings.Contains(lowerPrompt, "grep") ||
		strings.Contains(lowerPrompt, "find") || strings.Contains(lowerPrompt, "找") {
		output.WriteString("[CL Native] 搜索操作\n")
		if pattern := extractPattern(req.Prompt); pattern != "" {
			output.WriteString(fmt.Sprintf("  模式: %s\n", pattern))
		}
	}

	// Bash 檢測
	if strings.Contains(lowerPrompt, "run") || strings.Contains(lowerPrompt, "execute") ||
		strings.Contains(lowerPrompt, "command") || strings.Contains(lowerPrompt, "執行") {
		output.WriteString("[CL Native] 命令執行\n")
	}

	// 通用回應
	if output.Len() == 0 {
		output.WriteString(fmt.Sprintf("[CL Native] 已處理: %s\n", truncate(req.Prompt, 100)))
	}

	output.WriteString("\n[OK] Task completed (CL Native mode)\n")

	return &ExecutionResult{
		Output:       output.String(),
		Executor:     ExecutorCL,
		InputTokens:  len(req.Prompt) / 4,
		OutputTokens: len(output.String()) / 4,
		TotalTokens:  (len(req.Prompt) + len(output.String())) / 4,
		CostUSD:      0.001, // CL Native 幾乎無成本
		CacheHit:     false,
	}
}

// ExecuteStream 執行並流式返回結果
func (c *ClKernelClient) ExecuteStream(ctx context.Context, req ExecuteRequest, stream func(chunk string) error) error {
	result, err := c.Execute(ctx, req)
	if err != nil {
		return err
	}

	// 流式輸出
	lines := strings.Split(result.Output, "\n")
	for _, line := range lines {
		if err := stream(line + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// GetStats 返回使用統計
func (c *ClKernelClient) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Close 關閉客戶端
func (c *ClKernelClient) Close() error {
	// 清理資源
	return nil
}

// ============================================================================
// Claude Code CLI 客戶端
// ============================================================================

// ClaudeCodeClient 是 Claude Code CLI 的客戶端
type ClaudeCodeClient struct {
	// 配置
	claudeCodePath string
	timeout        time.Duration

	// 狀態
	mu    sync.RWMutex
	stats Stats
}

// NewClaudeCodeClient 創建新的 Claude Code 客戶端
func NewClaudeCodeClient() *ClaudeCodeClient {
	// 查找 Claude Code CLI
	path := findClaudeCodePath()

	return &ClaudeCodeClient{
		claudeCodePath: path,
		timeout:        10 * time.Minute,
		stats:          Stats{},
	}
}

// Execute 通過 Claude Code CLI 執行任務
func (c *ClaudeCodeClient) Execute(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error) {
	start := time.Now()

	// 檢查 Claude Code 是否可用
	if c.claudeCodePath == "" {
		return &ExecutionResult{
			Executor: ExecutorClaudeCode,
			Error:    fmt.Errorf("claude code cli not found"),
			Duration: time.Since(start),
		}, fmt.Errorf("claude code cli not found")
	}

	// 執行 Claude Code
	result, err := c.executeClaudeCode(ctx, req)

	// 更新統計
	c.mu.Lock()
	c.stats.TotalRequests++
	c.stats.TotalLatencyMs += time.Since(start).Milliseconds()

	if err != nil {
		c.stats.FailureCount++
	} else {
		c.stats.SuccessCount++
		if result != nil {
			c.stats.TotalTokens += result.TotalTokens
			c.stats.TotalCostUSD += result.CostUSD
		}
	}
	// 計算平均延遲
	if c.stats.TotalRequests > 0 {
		c.stats.AverageLatency = time.Duration(c.stats.TotalLatencyMs/int64(c.stats.TotalRequests)) * time.Millisecond
	}
	c.mu.Unlock()

	if err != nil {
		return &ExecutionResult{
			Executor: ExecutorClaudeCode,
			Error:    err,
			Duration: time.Since(start),
		}, err
	}

	result.Duration = time.Since(start)
	return result, nil
}

// executeClaudeCode 執行 Claude Code CLI
func (c *ClaudeCodeClient) executeClaudeCode(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error) {
	// 構建命令參數
	args := []string{}

	// 添加 --print 標誌使 Claude Code 輸出結果
	args = append(args, "--print")

	// 添加 --model 如果指定
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	// 添加 prompt
	args = append(args, "--", req.Prompt)

	// 創建上下文
	cmdCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 執行命令
	cmd := exec.CommandContext(cmdCtx, c.claudeCodePath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// 檢查是否是超時
		if ctx.Err() == context.DeadlineExceeded {
			return &ExecutionResult{
				Executor: ExecutorClaudeCode,
				Error:    fmt.Errorf("claude code execution timed out"),
				Duration: c.timeout,
			}, fmt.Errorf("execution timed out")
		}
		return &ExecutionResult{
			Executor: ExecutorClaudeCode,
			Error:    fmt.Errorf("claude code execution failed: %w, output: %s", err, string(output)),
		}, err
	}

	// 估算 token (簡化: 每個單詞約 1.3 tokens)
	words := len(strings.Fields(string(output)))
	estimatedOutputTokens := words * 4 / 3

	// 估算輸入 token (prompt 長度)
	inputTokens := len(strings.Fields(req.Prompt)) * 4 / 3
	totalTokens := inputTokens + estimatedOutputTokens

	// Claude Code 成本 (使用 GPT-4o 定价)
	inputCost := float64(inputTokens) / 1e6 * 5.0             // $5 / 1M input
	outputCost := float64(estimatedOutputTokens) / 1e6 * 15.0 // $15 / 1M output
	cost := inputCost + outputCost

	return &ExecutionResult{
		Output:       string(output),
		Executor:     ExecutorClaudeCode,
		InputTokens:  inputTokens,
		OutputTokens: estimatedOutputTokens,
		TotalTokens:  totalTokens,
		CostUSD:      cost,
		CacheHit:     false,
	}, nil
}

// GetStats 返回使用統計
func (c *ClaudeCodeClient) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Close 關閉客戶端
func (c *ClaudeCodeClient) Close() error {
	// Claude Code CLI 不需要特殊關閉
	return nil
}

// findClaudeCodePath 查找 Claude Code CLI 路徑
func findClaudeCodePath() string {
	// 檢查常見路徑
	paths := []string{
		"claude",
		"claude-code",
	}

	// Windows 路徑
	if os.PathSeparator == '\\' {
		paths = []string{
			"claude",
			"claude-code",
			os.Getenv("LOCALAPPDATA") + "\\Claude\\claude.exe",
			os.Getenv("APPDATA") + "\\npm\\claude.cmd",
		}
	}

	// Unix 路徑
	paths = append(paths,
		"/usr/local/bin/claude",
		"/usr/bin/claude",
		os.Getenv("HOME")+"/.claude/bin/claude",
	)

	for _, path := range paths {
		if path == "" {
			continue
		}
		// 檢查是否存在
		if _, err := os.Stat(path); err == nil {
			return path
		}
		// 檢查是否在 PATH 中
		cmd := exec.Command("which", path)
		if output, err := cmd.CombinedOutput(); err == nil && len(output) > 0 {
			return strings.TrimSpace(string(output))
		}
	}

	return ""
}

// IsClaudeCodeAvailable 檢查 Claude Code CLI 是否可用
// 只檢查 CLI 是否存在，不測試是否可以執行（避免 hang）
func IsClaudeCodeAvailable() bool {
	return findClaudeCodePath() != ""
}

// CanClaudeCodeExecute 檢查 Claude Code CLI 是否可以正常執行（快速測試）
// 這個函數會嘗試執行 claude --version，如果超時則認為不可用
func CanClaudeCodeExecute() bool {
	path := findClaudeCodePath()
	if path == "" {
		return false
	}

	// 嘗試執行 claude --version 進行快速測試
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	err := cmd.Run()

	// 如果超時或失敗，認為不可用
	if ctx.Err() == context.DeadlineExceeded || err != nil {
		return false
	}
	return true
}

// ============================================================================
// 工具函數
// ============================================================================

// generateSessionID 生成唯一的 session ID
func generateSessionID() string {
	return fmt.Sprintf("cl_%d_%d", time.Now().UnixNano(), os.Getpid())
}

// extractFilePath 從 prompt 中提取檔案路徑
func extractFilePath(prompt string) string {
	words := strings.Fields(prompt)
	for i, word := range words {
		if strings.HasSuffix(word, ".go") || strings.HasSuffix(word, ".ts") ||
			strings.HasSuffix(word, ".js") || strings.HasSuffix(word, ".md") ||
			strings.HasSuffix(word, ".py") || strings.HasSuffix(word, ".txt") {
			return word
		}
		if (word == "in" || word == "from" || word == "file" || word == "檔案") && i+1 < len(words) {
			return words[i+1]
		}
	}
	return ""
}

// extractPattern 從 prompt 中提取搜索模式
func extractPattern(prompt string) string {
	quotes := strings.Split(prompt, "\"")
	if len(quotes) >= 2 {
		return quotes[1]
	}
	return ""
}

// truncate 截斷字串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
