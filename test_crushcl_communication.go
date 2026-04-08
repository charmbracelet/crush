//go:build ignore
// +build ignore

// Standalone test to verify CrushCLAgentRunner HTTP communication
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

func main() {
	fmt.Println("=== CrushCL Agent Runner HTTP Communication Test ===\n")

	// Test 1: Mock server simulating CrushCL kernel server
	fmt.Println("Test 1: CrushCLAgentRunner -> Mock Kernel Server")
	testWithMockServer()

	// Test 2: Direct MiniMax API call
	fmt.Println("\nTest 2: Direct MiniMax API Call")
	testDirectMinimaxAPI()
}

func testWithMockServer() {
	// Create mock server that simulates CrushCL kernel server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
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

	// Simulate CrushCLAgentRunner
	runner := &CrushCLAgentRunner{
		serverURL: mockServer.URL,
		apiKey:    "test-api-key",
		client:    &http.Client{Timeout: 10 * time.Second},
		executor:  "cl",
	}

	// Run test
	ctx := context.Background()
	result, err := runner.Run(ctx, "Hello, this is a test prompt")

	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		return
	}

	fmt.Printf("✅ PASS: Server communication successful\n")
	fmt.Printf("   Session ID: %s\n", result.SessionID)
	fmt.Printf("   Response: %s\n", result.Text)
	fmt.Printf("   Tokens: %d, Cost: $%.6f\n", result.Tokens, result.CostUSD)
}

func testDirectMinimaxAPI() {
	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	// Build request
	reqBody := map[string]interface{}{
		"model": "MiniMax-M2.7-highspeed",
		"messages": []map[string]string{
			{"role": "user", "content": "Say hello in 10 words or less"},
		},
		"max_tokens": 100,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("❌ FAIL: Failed to marshal request: %v\n", err)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST",
		"https://api.minimax.io/anthropic/v1/messages",
		bytes.NewReader(payloadBytes))
	if err != nil {
		fmt.Printf("❌ FAIL: Failed to create request: %v\n", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ FAIL: Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ FAIL: API returned status %d: %s\n", resp.StatusCode, string(body))
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("❌ FAIL: Failed to parse response: %v\n", err)
		return
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

	fmt.Printf("✅ PASS: Direct MiniMax API call successful\n")
	fmt.Printf("   Response: %s\n", text)
	fmt.Printf("   Usage: %d input tokens, %d output tokens\n", inputTokens, outputTokens)
}

// CrushCLAgentRunner simulates the CrushCLAgentRunner implementation
type CrushCLAgentRunner struct {
	serverURL string
	apiKey    string
	client    *http.Client
	executor  string
}

type AgentResult struct {
	SessionID string
	Text      string
	Tokens    int
	CostUSD   float64
}

func (r *CrushCLAgentRunner) Run(ctx context.Context, prompt string) (*AgentResult, error) {
	reqBody := map[string]interface{}{
		"prompt":   prompt,
		"tools":    []string{},
		"executor": r.executor,
		"model":    "MiniMax-M2.7-highspeed",
		"stream":   false,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := r.serverURL + "/api/v1/execute"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
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
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &AgentResult{
		SessionID: serverResp.SessionID,
		Text:      serverResp.Text,
		Tokens:    serverResp.Tokens,
		CostUSD:   serverResp.CostUSD,
	}, nil
}
