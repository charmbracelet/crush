//go:build ignore

// This is a standalone test script to verify the reasoning_text → reasoning_content
// field transformation applied by the copilot transport.
// Run with: go run scripts/test_thinking.go
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth/copilot"
)

const (
	baseURL = "https://copilot-api.999gml.xyz/"
	apiKey  = "sk-7a5be203-30e4-44c4-9f41-1d876111ca44"
	modelID = "claude-sonnet-4.6"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	MaxTokens       int           `json:"max_tokens"`
	Stream          bool          `json:"stream"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

func main() {
	fmt.Println("=== Testing Copilot API reasoning_text → reasoning_content Transform ===")
	fmt.Println()
	fmt.Println("This test verifies that the copilot transport correctly renames")
	fmt.Println(`"reasoning_text" to "reasoning_content" in the SSE stream.`)
	fmt.Println()

	fmt.Println("--- Using copilot.NewClient (with reasoning transform) ---")
	testWithClient(copilot.NewClient(false, false))

	fmt.Println()

	fmt.Println("--- Using plain http.DefaultClient (raw API response) ---")
	testWithClient(&http.Client{Timeout: 60 * time.Second})
}

func testWithClient(client *http.Client) {
	req := ChatRequest{
		Model: modelID,
		Messages: []ChatMessage{
			{Role: "user", Content: "What is 2+2?"},
		},
		MaxTokens:       300,
		Stream:          true,
		ReasoningEffort: "high",
	}

	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", baseURL+"v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	chunkCount := 0
	reasoningContentCount := 0
	reasoningTextCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		chunkCount++

		if choices, ok := chunk["choices"].([]any); ok {
			for _, choice := range choices {
				c, ok := choice.(map[string]any)
				if !ok {
					continue
				}
				if delta, ok := c["delta"].(map[string]any); ok {
					if v, ok := delta["reasoning_content"]; ok && v != "" {
						reasoningContentCount++
					}
					if v, ok := delta["reasoning_text"]; ok && v != "" {
						reasoningTextCount++
					}
				}
			}
		}
	}

	fmt.Printf("Total chunks: %d\n", chunkCount)
	fmt.Printf("  reasoning_content chunks (standard, what crush expects): %d\n", reasoningContentCount)
	fmt.Printf("  reasoning_text chunks (non-standard, raw API): %d\n", reasoningTextCount)

	if reasoningContentCount > 0 && reasoningTextCount == 0 {
		fmt.Println("  ✓ Transform working correctly: reasoning_text → reasoning_content")
	} else if reasoningTextCount > 0 && reasoningContentCount == 0 {
		fmt.Println("  ✗ Transform NOT applied: still seeing reasoning_text")
	} else if reasoningContentCount == 0 && reasoningTextCount == 0 {
		fmt.Println("  ✗ No reasoning content received at all")
	}
}
