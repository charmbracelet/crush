package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// Updated to match VSCode Copilot Chat versions from captured traffic
	userAgent           = "GitHubCopilotChat/0.35.2"
	editorVersion       = "vscode/1.107.1"
	editorPluginVersion = "copilot-chat/0.35.2"
	integrationID       = "vscode-chat"
)

func Headers() map[string]string {
	return map[string]string{
		"User-Agent":             userAgent,
		"Editor-Version":         editorVersion,
		"Editor-Plugin-Version":  editorPluginVersion,
		"Copilot-Integration-Id": integrationID,
	}
}

// ParseEndpointFromToken extracts the API endpoint from a Copilot token.
// The token format is: "tid=xxx;exp=xxx;proxy-ep=proxy.individual.githubcopilot.com;..."
// Returns the HTTPS API endpoint URL, or empty string if not found.
func ParseEndpointFromToken(token string) string {
	// Token fields are separated by semicolons
	parts := strings.Split(token, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "proxy-ep=") {
			proxyEP := strings.TrimPrefix(part, "proxy-ep=")
			// Convert proxy endpoint to API endpoint
			// proxy.individual.githubcopilot.com -> api.individual.githubcopilot.com
			if strings.HasPrefix(proxyEP, "proxy.") {
				apiEP := "api." + strings.TrimPrefix(proxyEP, "proxy.")
				return "https://" + apiEP
			}
			return "https://" + proxyEP
		}
	}
	return ""
}

// SessionResponse represents the response from /models/session endpoint.
type SessionResponse struct {
	AvailableModels []string           `json:"available_models"`
	SelectedModel   string             `json:"selected_model"`
	SessionToken    string             `json:"session_token"`
	ExpiresAt       int64              `json:"expires_at"`
	DiscountedCosts map[string]float64 `json:"discounted_costs"`
}

// InitAutoModeSession calls /models/session to establish a discounted session.
// This enables 0.1 quota per interaction instead of 1.0.
// baseURL should be the Copilot API endpoint (e.g., https://api.individual.githubcopilot.com)
// token is the Bearer token for authorization.
func InitAutoModeSession(ctx context.Context, baseURL, token string, headers map[string]string) (*SessionResponse, error) {
	url := strings.TrimSuffix(baseURL, "/") + "/models/session"

	// Request body for auto mode
	reqBody := map[string]any{
		"auto_mode": map[string]any{
			"model_hints": []string{"auto"},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call /models/session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("session call failed with status %d: %s", resp.StatusCode, string(body))
	}

	var sessionResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &sessionResp, nil
}
