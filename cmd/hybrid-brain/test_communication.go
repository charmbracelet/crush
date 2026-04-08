//go:build ignore
// +build ignore

// Test script to verify CrushCLAgentRunner communicates with CrushCL kernel server
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/charmbracelet/crushcl/cmd/hybrid-brain/internal/kernel/cl_kernel"
)

// MockAgentRunner implements AgentRunner for testing
type MockAgentRunner struct{}

func (m *MockAgentRunner) Run(ctx context.Context, call cl_kernel.AgentCall) (*cl_kernel.AgentResult, error) {
	return &cl_kernel.AgentResult{
		Response: cl_kernel.AgentResponse{
			Content: cl_kernel.AgentResponseContent{
				Text: fmt.Sprintf("Mock response to: %s", call.Prompt),
			},
		},
		TotalUsage: cl_kernel.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}, nil
}

func main() {
	fmt.Println("=== CrushCL Agent Runner Communication Test ===\n")

	// Test 1: Test CrushCLAgentRunner with httptest server
	fmt.Println("Test 1: CrushCLAgentRunner -> Mock HTTP Server")
	testWithMockServer()

	// Test 2: Direct API call simulation
	fmt.Println("\nTest 2: CrushCLAgentRunner -> Simulated Real Server")
	testSimulatedRealServer()
}

func testWithMockServer() {
	// Create mock server that simulates CrushCL kernel server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "Missing authorization", http.StatusUnauthorized)
			return
		}

		// Check content type
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Invalid content type", http.StatusBadRequest)
			return
		}

		// Parse request
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Return mock response
		resp := map[string]interface{}{
			"session_id":  "test-session-123",
			"text":        fmt.Sprintf("Processed: %v", req["prompt"]),
			"tokens":      150,
			"cost_usd":    0.001,
			"executor":    "cl",
			"duration_ms": 100,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create runner with mock server URL
	runner := &CrushCLAgentRunnerTest{
		serverURL: mockServer.URL,
		apiKey:    "test-api-key",
		client:    &http.Client{Timeout: 10 * time.Second},
		executor:  "cl",
	}

	// Run test
	ctx := context.Background()
	call := cl_kernel.AgentCall{
		SessionID:       "test-session",
		Prompt:          "Hello, this is a test",
		MaxOutputTokens: 1024,
		SystemPrompt:    "You are a helpful assistant",
	}

	result, err := runner.Run(ctx, call)
	if err != nil {
		fmt.Printf("❌ FAIL: %v\n", err)
		return
	}

	fmt.Printf("✅ PASS: Got response: %s\n", result.Response.Content.Text)
	fmt.Printf("   Tokens used: %d input, %d output\n",
		result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens)
}

func testSimulatedRealServer() {
	// Simulate what happens when calling the real server
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		apiKey = "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"
	}

	// Build request like CrushCLAgentRunner does
	reqBody := map[string]interface{}{
		"prompt":   "Say hello in 10 words or less",
		"tools":    []string{},
		"executor": "cl",
		"model":    "MiniMax-M2.7-highspeed",
		"stream":   false,
	}

	payloadBytes, _ := json.Marshal(reqBody)

	// Create request to real MiniMax API
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

	// Extract text content
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

	fmt.Printf("✅ PASS: Real API call successful\n")
	fmt.Printf("   Response: %s\n", text)
}

// CrushCLAgentRunnerTest is a test version of CrushCLAgentRunner
type CrushCLAgentRunnerTest struct {
	serverURL string
	apiKey    string
	client    *http.Client
	executor  string
}

func (r *CrushCLAgentRunnerTest) Run(ctx context.Context, call cl_kernel.AgentCall) (*cl_kernel.AgentResult, error) {
	reqBody := map[string]interface{}{
		"prompt":   call.Prompt,
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
