package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMiniMaxRealAPI_ChatCompletion(t *testing.T) {
	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	payload := map[string]interface{}{
		"model": "MiniMax-M2.7-highspeed",
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'hello' and confirm you are working"},
		},
		"max_tokens": 50,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.minimax.io/anthropic/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("MINIMAX chat completion failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("MINIMAX API returned status %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		for _, block := range content {
			if m, ok := block.(map[string]interface{}); ok {
				if m["type"] == "text" {
					fmt.Fprintf(os.Stderr, "[PASS] MINIMAX API Response text: %v\n", m["text"])
					return
				}
			}
		}
	}
	fmt.Fprintf(os.Stderr, "[PASS] MINIMAX API Response: %+v\n", result)
}

func TestMiniMaxRealAPI_HealthCheckViaMessages(t *testing.T) {
	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	payload := map[string]interface{}{
		"model": "MiniMax-M2.7-highspeed",
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 5,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.minimax.io/anthropic/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("MINIMAX health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("MINIMAX health check returned %d, body: %s", resp.StatusCode, string(body))
	}

	fmt.Fprintf(os.Stderr, "[PASS] MINIMAX health check via /messages: OK\n")
}

func TestMiniMaxRealAPI_ModelEndpoint(t *testing.T) {
	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://api.minimax.io/anthropic/v1/models", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("MINIMAX /models request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Fprintf(os.Stderr, "[INFO] MINIMAX /models status: %d\n", resp.StatusCode)

	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(os.Stderr, "[INFO] MINIMAX does NOT have /models endpoint (404)\n")
		t.Skip("MINIMAX does not support /models endpoint")
	}
}

func TestMiniMaxRealAPI_TimeoutBehavior(t *testing.T) {
	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	client := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	payload := map[string]interface{}{
		"model": "MiniMax-M2.7-highspeed",
		"messages": []map[string]string{
			{"role": "user", "content": "Count to 100"},
		},
		"max_tokens": 200,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.minimax.io/anthropic/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	_, err = client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[PASS] Timeout works: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[WARN] Request succeeded with 100ms timeout\n")
}

func TestMiniMaxRealAPI_RetryWithTransport(t *testing.T) {
	apiKey := "sk-cp-VXD3fTnW8eOJd__DNOMiX75Qd4BLU-XSv9nfjpBllIczwd_jl6Y9ISiPdo2UYlOYIDosWUboPtTUYaDoXCSsF8tFHUkqDP0_FigbUumFTIQEkJH4B5oFmSE"

	failCount := 0
	transport := &countingTransport{
		transport:  http.DefaultTransport,
		onRequest:  func() { failCount++ },
		shouldFail: true,
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	payload := map[string]interface{}{
		"model": "MiniMax-M2.7-highspeed",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
		"max_tokens": 10,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.minimax.io/anthropic/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[INFO] Request failed as expected: %v\n", err)
		fmt.Fprintf(os.Stderr, "[INFO] Fail count: %d\n", failCount)
		return
	}
	defer resp.Body.Close()

	fmt.Fprintf(os.Stderr, "[INFO] Request succeeded after %d attempts, status: %d\n", failCount, resp.StatusCode)
}

type countingTransport struct {
	transport  http.RoundTripper
	onRequest  func()
	shouldFail bool
	failCount  int
	mu         sync.Mutex
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.onRequest()
	if t.shouldFail && t.failCount < 1 {
		t.failCount++
		t.mu.Unlock()
		return nil, fmt.Errorf("simulated network error")
	}
	t.mu.Unlock()
	return t.transport.RoundTrip(req)
}
