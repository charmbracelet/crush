package narrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Agent handles communication with Ollama API
type Agent struct {
	baseURL    string
	model      string
	timeout    time.Duration
	enabled    bool
	httpClient *http.Client
}

// OllamaRequest represents a request to Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// OllamaResponse represents a response from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewAgent creates a new Ollama agent
func NewAgent(baseURL, model string, timeout time.Duration) *Agent {
	return &Agent{
		baseURL: baseURL,
		model:   model,
		timeout: timeout,
		enabled: true,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Explain generates an explanation for the given context
func (a *Agent) Explain(ctx context.Context, prompt string) (string, error) {
	if !a.enabled {
		return a.getOfflineMessage(), nil
	}

	request := OllamaRequest{
		Model:  a.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Sprintf("Error creating request: %v", err), nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Sprintf("Error creating request: %v", err), nil
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return a.getOfflineMessage(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Ollama API error: %s", resp.Status), nil
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return fmt.Sprintf("Error parsing response: %v", err), nil
	}

	return ollamaResp.Response, nil
}

// ExplainStream generates a streaming explanation
func (a *Agent) ExplainStream(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string, 10)

	go func() {
		defer close(ch)

		if !a.enabled {
			ch <- a.getOfflineMessage()
			return
		}

		request := OllamaRequest{
			Model:  a.model,
			Prompt: prompt,
			Stream: true,
		}

		jsonData, err := json.Marshal(request)
		if err != nil {
			ch <- fmt.Sprintf("Error creating request: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
		if err != nil {
			ch <- fmt.Sprintf("Error creating request: %v", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := a.httpClient.Do(req)
		if err != nil {
			ch <- a.getOfflineMessage()
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			ch <- fmt.Sprintf("Ollama API error: %s", resp.Status)
			return
		}

		decoder := json.NewDecoder(resp.Body)
		for {
			var partial struct {
				Response string `json:"response,omitempty"`
				Done     bool   `json:"done,omitempty"`
			}

			if err := decoder.Decode(&partial); err != nil {
				break
			}

			if partial.Response != "" {
				ch <- partial.Response
			}

			if partial.Done {
				break
			}
		}
	}()

	return ch, nil
}

// IsEnabled returns whether the agent is enabled
func (a *Agent) IsEnabled() bool {
	return a.enabled
}

// SetEnabled enables or disables the agent
func (a *Agent) SetEnabled(enabled bool) {
	a.enabled = enabled
}

// SetModel updates the model used by the agent
func (a *Agent) SetModel(model string) {
	a.model = model
}

// SetBaseURL updates the base URL for the Ollama API
func (a *Agent) SetBaseURL(url string) {
	a.baseURL = url
}

// getOfflineMessage returns a fallback message when Ollama is unavailable
func (a *Agent) getOfflineMessage() string {
	return "AI narrator is offline. Please ensure Ollama is running and accessible."
}

// TestConnection tests if Ollama is accessible
func (a *Agent) TestConnection(ctx context.Context) error {
	if !a.enabled {
		return fmt.Errorf("agent is disabled")
	}

	// Simple health check request
	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API returned status: %s", resp.Status)
	}

	return nil
}

// GetAvailableModels returns a list of available models from Ollama
func (a *Agent) GetAvailableModels(ctx context.Context) ([]string, error) {
	if !a.enabled {
		return []string{}, fmt.Errorf("agent is disabled")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API returned status: %s", resp.Status)
	}

	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]string, len(response.Models))
	for i, model := range response.Models {
		models[i] = model.Name
	}

	return models, nil
}
