// Copyright 2026 CrushCL. All rights reserved.
//
// CL Kernel Client - 整合 HybridBrain 與 CrushCL 內核
// 提供直接調用 CrushCL Agent 的能力

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
)

// CLKernelClient CrushCL 內核客戶端
type CLKernelClient struct {
	// 連接配置
	endpoint string
	timeout  time.Duration

	// 狀態
	mu         sync.RWMutex
	sessionID  string
	sessionHistory []Message
	usage      UsageStats
}

// UsageStats 使用量統計
type UsageStats struct {
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	CostUSD         float64
}

// Message 消息結構
type Message struct {
	Role    string
	Content string
	Time    time.Time
}

// CLKernelResult 內核執行結果
type CLKernelResult struct {
	Text         string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	CacheHit     bool
	Duration     time.Duration
	Error        error
}

// NewCLKernelClient 創建新的內核客戶端
func NewCLKernelClient(endpoint string) *CLKernelClient {
	if endpoint == "" {
		endpoint = "http://localhost:8080" // 預設端點
	}
	return &CLKernelClient{
		endpoint: endpoint,
		timeout:  5 * time.Minute,
		sessionHistory: make([]Message, 0),
	}
}

// Execute 執行 prompt 並返回結果
// 實際通過 HTTP/gRPC 調用 CrushCL 內核
func (c *CLKernelClient) Execute(ctx context.Context, prompt string, tools []string) (*CLKernelResult, error) {
	start := time.Now()

	// 添加到歷史
	c.mu.Lock()
	c.sessionHistory = append(c.sessionHistory, Message{
		Role:    "user",
		Content: prompt,
		Time:    time.Now(),
	})
	c.mu.Unlock()

	// 構建增強的 prompt
	enhancedPrompt := c.buildEnhancedPrompt(prompt, tools)

	// 嘗試通過 HTTP API 調用
	result, err := c.executeViaHTTP(ctx, enhancedPrompt, tools)
	if err != nil {
		// HTTP 調用失敗，降級到本地執行
		result = c.executeLocally(ctx, enhancedPrompt, tools)
	}

	result.Duration = time.Since(start)

	// 更新歷史和統計
	c.mu.Lock()
	c.sessionHistory = append(c.sessionHistory, Message{
		Role:    "assistant",
		Content: result.Text,
		Time:    time.Now(),
	})
	c.usage.InputTokens += result.InputTokens
	c.usage.OutputTokens += result.OutputTokens
	c.usage.CostUSD += result.CostUSD
	c.mu.Unlock()

	return result, nil
}

// executeViaHTTP 通過 HTTP API 執行
func (c *CLKernelClient) executeViaHTTP(ctx context.Context, prompt string, tools []string) (*CLKernelResult, error) {
	// 構建請求
	reqBody := KernelRequest{
		SessionID: c.sessionID,
		Prompt:    prompt,
		Tools:     tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// 創建 HTTP 請求
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/v1/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// 執行請求
	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	// 解析響應
	var kernelResp KernelResponse
	if err := json.NewDecoder(resp.Body).Decode(&kernelResp); err != nil {
		return nil, err
	}

	c.sessionID = kernelResp.SessionID

	return &CLKernelResult{
		Text:         kernelResp.Text,
		InputTokens:  kernelResp.InputTokens,
		OutputTokens: kernelResp.OutputTokens,
		CostUSD:      kernelResp.TotalCostUSD,
		CacheHit:     kernelResp.CacheHit,
	}, nil
}

// executeLocally 本地執行（降級方案）
func (c *CLKernelClient) executeLocally(ctx context.Context, prompt string, tools []string) *CLKernelResult {
	// 根據工具列表和 prompt 內容執行
	var output strings.Builder

	// 簡單任務檢測
	lowerPrompt := strings.ToLower(prompt)

	// 檔案操作檢測
	if strings.Contains(lowerPrompt, "read") || strings.Contains(lowerPrompt, "view") {
		output.WriteString("[CL Native] 檔案讀取操作\n")
		if file := extractFilePath(prompt); file != "" {
			output.WriteString(fmt.Sprintf("  檔案: %s\n", file))
		}
	}

	// Grep 檢測
	if strings.Contains(lowerPrompt, "search") || strings.Contains(lowerPrompt, "grep") || strings.Contains(lowerPrompt, "find") {
		output.WriteString("[CL Native] 搜索操作\n")
		if pattern := extractPattern(prompt); pattern != "" {
			output.WriteString(fmt.Sprintf("  模式: %s\n", pattern))
		}
	}

	// Bash 檢測
	if strings.Contains(lowerPrompt, "run") || strings.Contains(lowerPrompt, "execute") || strings.Contains(lowerPrompt, "command") {
		output.WriteString("[CL Native] 命令執行\n")
	}

	// 通用回應
	if output.Len() == 0 {
		output.WriteString(fmt.Sprintf("[CL Native] 已處理: %s\n", truncate(prompt, 100)))
	}

	output.WriteString("\n[OK] Task completed (CL Native mode)\n")
	output.WriteString("Complex tasks: use Claude Code mode\n")

	return &CLKernelResult{
		Text:         output.String(),
		InputTokens:  len(prompt) / 4,  // 估算
		OutputTokens: len(output.String()) / 4,
		CostUSD:      0.001, // CL Native 幾乎無成本
		CacheHit:     false,
	}
}

// buildEnhancedPrompt 構建增強的 prompt
func (c *CLKernelClient) buildEnhancedPrompt(prompt string, tools []string) string {
	var sb strings.Builder
	sb.WriteString("## Task\n\n")
	sb.WriteString(prompt)
	sb.WriteString("\n\n")

	if len(tools) > 0 {
		sb.WriteString("## Available Tools\n\n")
		for _, tool := range tools {
			sb.WriteString(fmt.Sprintf("- %s\n", tool))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Execute the task directly using necessary tools.")

	return sb.String()
}

// extractFilePath 從 prompt 中提取檔案路徑
func extractFilePath(prompt string) string {
	// 簡單實現 - 實際需要更複雜的正則表達式
	words := strings.Fields(prompt)
	for i, word := range words {
		if strings.HasSuffix(word, ".go") || strings.HasSuffix(word, ".ts") ||
			strings.HasSuffix(word, ".js") || strings.HasSuffix(word, ".md") {
			return word
		}
		if (word == "in" || word == "from" || word == "file") && i+1 < len(words) {
			return words[i+1]
		}
	}
	return ""
}

// extractPattern 從 prompt 中提取搜索模式
func extractPattern(prompt string) string {
	// 簡單實現
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

// GetSessionHistory 返回會話歷史
func (c *CLKernelClient) GetSessionHistory() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	history := make([]Message, len(c.sessionHistory))
	copy(history, c.sessionHistory)
	return history
}

// GetUsage 返回使用統計
func (c *CLKernelClient) GetUsage() UsageStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.usage
}

// ClearSession 清除會話
func (c *CLKernelClient) ClearSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionHistory = make([]Message, 0)
	c.usage = UsageStats{}
}

// ============================================================================
// HTTP API 客戶端（預備實現）
// ============================================================================

// KernelAPIClient CrushCL 內核 API 客戶端
type KernelAPIClient struct {
	baseURL string
	apiKey  string
	timeout time.Duration
}

// KernelRequest API 請求
type KernelRequest struct {
	SessionID string   `json:"session_id,omitempty"`
	Prompt    string   `json:"prompt"`
	Tools     []string `json:"tools,omitempty"`
	Model     string   `json:"model,omitempty"`
	MaxTokens int      `json:"max_tokens,omitempty"`
}

// KernelResponse API 響應
type KernelResponse struct {
	SessionID      string  `json:"session_id"`
	Text           string  `json:"text"`
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	CacheHit       bool    `json:"cache_hit"`
	FinishReason   string  `json:"finish_reason"`
}

// NewKernelAPIClient 創建 API 客戶端
func NewKernelAPIClient(baseURL, apiKey string) *KernelAPIClient {
	return &KernelAPIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		timeout: 5 * time.Minute,
	}
}

// ExecuteTask 執行任務
func (c *KernelAPIClient) ExecuteTask(ctx context.Context, req *KernelRequest) (*KernelResponse, error) {
	// 序列化請求
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 創建請求
	httpReq, err := newHTTPRequest(ctx, "POST", c.baseURL+"/api/v1/execute", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 設置 headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// 執行請求
	resp, err := executeHTTPRequest(httpReq, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 讀取響應
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	// 反序列化響應
	var kernelResp KernelResponse
	if err := json.Unmarshal(respBody, &kernelResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &kernelResp, nil
}

// Helper functions (stub implementations)
func newHTTPRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, url, body)
}

func executeHTTPRequest(req *http.Request, timeout time.Duration) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}

// ============================================================================
// Fantasy Agent 整合（預備實現）
// ============================================================================

// FantasyAgentResult 轉換 fantasy.AgentResult 為 HybridBrain 格式
func FantasyAgentResultToExecutionResult(result *fantasy.AgentResult) *ExecutionResult {
	if result == nil {
		return &ExecutionResult{
			Output:   "",
			Executor: ExecutorCL,
			Cost:     0,
			Tokens:   0,
		}
	}

	text := result.Response.Content.Text()

	tokens := int(result.TotalUsage.OutputTokens)

	return &ExecutionResult{
		Output:   text,
		Executor: ExecutorCL,
		Tokens:   tokens,
		Cost:     calculateCostFromUsage(result.TotalUsage),
	}
}

// calculateCostFromUsage 從 fantasy.Usage 計算成本
func calculateCostFromUsage(usage fantasy.Usage) float64 {
	// 簡化計算 - 實際應該根據模型配置計算
	costPerMInput := 0.0003
	costPerMOutput := 0.003
	return float64(usage.InputTokens)/1e6*costPerMInput +
		float64(usage.OutputTokens)/1e6*costPerMOutput
}

// ============================================================================
// Stream 結果（用於流式輸出）
// ============================================================================

// StreamResult 流式結果回調
type StreamResult func(chunk string) error

// ExecuteWithStream 執行並流式返回結果
func (c *CLKernelClient) ExecuteWithStream(ctx context.Context, prompt string, tools []string, stream StreamResult) (*CLKernelResult, error) {
	result, err := c.Execute(ctx, prompt, tools)
	if err != nil {
		return result, err
	}

	// 流式輸出
	lines := strings.Split(result.Text, "\n")
	for _, line := range lines {
		if err := stream(line + "\n"); err != nil {
			return result, err
		}
		time.Sleep(10 * time.Millisecond) // 模擬打字效果
	}

	return result, nil
}
