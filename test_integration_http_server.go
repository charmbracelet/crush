//go:build ignore
// +build ignore

// Integration test for CrushCL Kernel HTTP Server with real AgentRunner
// This test verifies end-to-end communication between HTTP client and server
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/charmbracelet/crushcl/internal/kernel/cl_kernel"
	"github.com/charmbracelet/crushcl/internal/kernel/server"
)

// MiniMaxAgentRunner implements cl_kernel.AgentRunner by calling MiniMax API
type MiniMaxAgentRunner struct {
	apiKey string
}

func (r *MiniMaxAgentRunner) Run(ctx context.Context, call cl_kernel.AgentCall) (*cl_kernel.AgentResult, error) {
	// Build request to MiniMax API
	reqBody := map[string]interface{}{
		"model": "MiniMax-M2.7-highspeed",
		"messages": []map[string]string{
			{"role": "user", "content": call.Prompt},
		},
		"max_tokens": int(call.MaxOutputTokens),
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := "https://api.minimax.io/anthropic/v1/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call MiniMax API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &cl_kernel.AgentResult{
			Response: cl_kernel.AgentResponse{
				Content: cl_kernel.AgentResponseContent{
					Text: fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
				},
			},
		}, nil
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract content
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

	// Extract usage
	var inputTokens, outputTokens int
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

func main() {
	fmt.Println("=== CrushCL Kernel HTTP Server Integration Test ===\n")

	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	// Test 1: Mock server test (sanity check)
	fmt.Println("Test 1: Mock Server (Sanity Check)")
	testMockServer()

	// Test 2: Real integration with MiniMax API through HTTP server
	fmt.Println("\nTest 2: Real Integration via HTTP Server")
	testRealIntegration(apiKey)
}

func testMockServer() {
	// Create mock server that simulates the CrushCL kernel server behavior
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, `{"error": "Missing authorization"}`, http.StatusUnauthorized)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, `{"error": "Invalid content type"}`, http.StatusBadRequest)
			return
		}

		var req struct {
			Prompt   string   `json:"prompt"`
			Tools    []string `json:"tools"`
			Executor string   `json:"executor"`
			Model    string   `json:"model"`
			Stream   bool     `json:"stream"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
			return
		}

		// Simulate CrushCL processing
		resp := map[string]interface{}{
			"session_id":  "test-session-123",
			"text":        fmt.Sprintf("Processed by CrushCL kernel: %s", req.Prompt),
			"tokens":      len(req.Prompt) / 4,
			"cost_usd":    0.001,
			"executor":    req.Executor,
			"duration_ms": 50,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create HTTP client to call mock server
	client := &http.Client{Timeout: 10 * time.Second}

	reqBody := map[string]interface{}{
		"prompt":   "Hello, this is a test prompt",
		"tools":    []string{},
		"executor": "cl",
		"model":    "MiniMax-M2.7-highspeed",
		"stream":   false,
	}

	payloadBytes, _ := json.Marshal(reqBody)
	url := mockServer.URL + "/api/v1/execute"
	req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(payloadBytes))
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ FAIL: Server returned status %d: %s\n", resp.StatusCode, string(body))
		return
	}

	var serverResp struct {
		SessionID  string  `json:"session_id"`
		Text       string  `json:"text"`
		Tokens     int     `json:"tokens"`
		CostUSD    float64 `json:"cost_usd"`
		Executor   string  `json:"executor"`
		DurationMs int64   `json:"duration_ms"`
	}

	if err := json.Unmarshal(body, &serverResp); err != nil {
		fmt.Printf("❌ FAIL: Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("✅ PASS: Mock server communication successful\n")
	fmt.Printf("   Session ID: %s\n", serverResp.SessionID)
	fmt.Printf("   Response: %s\n", serverResp.Text)
	fmt.Printf("   Tokens: %d, Cost: $%.6f\n", serverResp.Tokens, serverResp.CostUSD)
}

func testRealIntegration(apiKey string) {
	// Create the MiniMax agent runner
	runner := &MiniMaxAgentRunner{apiKey: apiKey}

	// Start server in background
	serverAddr := "localhost:18080"

	httpServer_custom := server.NewServerWithAgent(runner, server.ServerConfig{
		Host:         "localhost",
		Port:         18080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	// Start in goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", serverAddr)
		if err := httpServer_custom.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Make HTTP request to the server
	client := &http.Client{Timeout: 60 * time.Second}

	reqBody := map[string]interface{}{
		"prompt":   "Say 'Hello from CrushCL HTTP Server' in exactly those words",
		"tools":    []string{},
		"executor": "cl",
		"model":    "MiniMax-M2.7-highspeed",
		"stream":   false,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("❌ FAIL: Failed to marshal request: %v\n", err)
		return
	}

	url := fmt.Sprintf("http://%s/api/v1/execute", serverAddr)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		fmt.Printf("❌ FAIL: Failed to create request: %v\n", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	fmt.Printf("   Making request to %s...\n", url)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ FAIL: Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ FAIL: Failed to read response: %v\n", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ FAIL: Server returned status %d: %s\n", resp.StatusCode, string(body))
		return
	}

	var serverResp struct {
		SessionID  string  `json:"session_id"`
		Text       string  `json:"text"`
		Tokens     int     `json:"tokens"`
		CostUSD    float64 `json:"cost_usd"`
		Executor   string  `json:"executor"`
		DurationMs int64   `json:"duration_ms"`
	}

	if err := json.Unmarshal(body, &serverResp); err != nil {
		fmt.Printf("❌ FAIL: Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("✅ PASS: Real HTTP server integration successful\n")
	fmt.Printf("   Session ID: %s\n", serverResp.SessionID)
	fmt.Printf("   Response: %s\n", serverResp.Text)
	fmt.Printf("   Tokens: %d, Cost: $%.6f, Duration: %dms\n",
		serverResp.Tokens, serverResp.CostUSD, serverResp.DurationMs)
	fmt.Printf("   Executor: %s\n", serverResp.Executor)
}
