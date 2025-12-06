// Package copilot provides token management for GitHub Copilot integration.
// It reads OAuth tokens from existing IDE installations (VSCode/JetBrains)
// and exchanges them for short-lived Copilot API tokens.
package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/google/uuid"
)

// VSCode's OAuth client ID - used as the key in apps.json.
const vscodeClientID = "Iv1.b507a08c87ecfe98"

// GitHub API endpoints.
const (
	githubAPIBaseURL = "https://api.github.com"
)

// Copilot API configuration.
const (
	copilotVersion       = "0.26.7"
	editorPluginVersion  = "copilot-chat/" + copilotVersion
	userAgent            = "GitHubCopilotChat/" + copilotVersion
	apiVersion           = "2025-04-01"
	defaultVSCodeVersion = "1.100.0"
)

// AppsEntry represents a single entry in the apps.json file.
type AppsEntry struct {
	User        string `json:"user"`
	OAuthToken  string `json:"oauth_token"`
	GitHubAppID string `json:"githubAppId"`
}

// CopilotToken represents a GitHub Copilot API token.
type CopilotToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	RefreshIn int64  `json:"refresh_in"`
}

// IsExpired checks if the Copilot token is expired or about to expire.
func (t *CopilotToken) IsExpired() bool {
	// Add 60 second buffer before expiration.
	return time.Now().Unix() >= (t.ExpiresAt - 60)
}

// AppsJSONPath returns the path to the GitHub Copilot apps.json file.
func AppsJSONPath() string {
	var configDir string
	switch runtime.GOOS {
	case "windows":
		configDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "github-copilot")
	default:
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config", "github-copilot")
	}
	return filepath.Join(configDir, "apps.json")
}

// ReadGitHubToken reads the GitHub OAuth token from the IDE's apps.json file.
// It looks for the VSCode client ID entry.
func ReadGitHubToken() (string, error) {
	path := AppsJSONPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read apps.json: %w", err)
	}

	var apps map[string]AppsEntry
	if err := json.Unmarshal(data, &apps); err != nil {
		return "", fmt.Errorf("failed to parse apps.json: %w", err)
	}

	// Look for VSCode's client ID entry.
	key := "github.com:" + vscodeClientID
	entry, ok := apps[key]
	if !ok {
		return "", fmt.Errorf("no VSCode Copilot token found in apps.json")
	}

	if entry.OAuthToken == "" {
		return "", fmt.Errorf("empty OAuth token in apps.json")
	}

	return entry.OAuthToken, nil
}

// HasGitHubToken checks if a valid GitHub token exists without reading it.
func HasGitHubToken() bool {
	path := AppsJSONPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var apps map[string]AppsEntry
	if err := json.Unmarshal(data, &apps); err != nil {
		return false
	}

	key := "github.com:" + vscodeClientID
	entry, ok := apps[key]
	return ok && entry.OAuthToken != ""
}

// GetCopilotToken exchanges a GitHub OAuth token for a Copilot API token.
func GetCopilotToken(ctx context.Context, githubToken string) (*CopilotToken, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIBaseURL+"/copilot_internal/v2/token", nil)
	if err != nil {
		return nil, err
	}

	setGitHubHeaders(req, githubToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get copilot token: status %d body %q", resp.StatusCode, string(data))
	}

	var result CopilotToken
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CopilotBaseURL returns the Copilot API base URL.
// Currently only supports individual accounts.
func CopilotBaseURL() string {
	return "https://api.githubcopilot.com"
}

// CopilotHeaders returns the headers required for Copilot API requests.
func CopilotHeaders(copilotToken string) map[string]string {
	return map[string]string{
		"Authorization":           "Bearer " + copilotToken,
		"Content-Type":            "application/json",
		"copilot-integration-id":  "vscode-chat",
		"editor-version":          "vscode/" + defaultVSCodeVersion,
		"editor-plugin-version":   editorPluginVersion,
		"User-Agent":              userAgent,
		"openai-intent":           "conversation-panel",
		"x-github-api-version":    apiVersion,
		"x-request-id":            uuid.New().String(),
		"x-vscode-user-agent-lib": "electron-fetch",
	}
}

func setGitHubHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("editor-version", "vscode/"+defaultVSCodeVersion)
	req.Header.Set("editor-plugin-version", editorPluginVersion)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("x-github-api-version", apiVersion)
}

// Model represents a Copilot model from the /models endpoint.
type Model struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Version            string           `json:"version"`
	Vendor             string           `json:"vendor"`
	Preview            bool             `json:"preview"`
	ModelPickerEnabled bool             `json:"model_picker_enabled"`
	Capabilities       ModelCapability  `json:"capabilities"`
	Policy             *ModelPolicy     `json:"policy,omitempty"`
}

// ModelCapability describes the model's capabilities and limits.
type ModelCapability struct {
	Family   string       `json:"family"`
	Type     string       `json:"type"`
	Tokenizer string      `json:"tokenizer"`
	Limits   ModelLimits  `json:"limits"`
	Supports ModelSupports `json:"supports"`
}

// ModelLimits defines token limits for the model.
type ModelLimits struct {
	MaxContextWindowTokens int `json:"max_context_window_tokens,omitempty"`
	MaxOutputTokens        int `json:"max_output_tokens,omitempty"`
	MaxPromptTokens        int `json:"max_prompt_tokens,omitempty"`
}

// ModelSupports describes what features the model supports.
type ModelSupports struct {
	ToolCalls         bool `json:"tool_calls,omitempty"`
	ParallelToolCalls bool `json:"parallel_tool_calls,omitempty"`
}

// ModelPolicy contains policy information for the model.
type ModelPolicy struct {
	State string `json:"state"`
	Terms string `json:"terms"`
}

// ModelsResponse is the response from the /models endpoint.
type ModelsResponse struct {
	Data   []Model `json:"data"`
	Object string  `json:"object"`
}

// GetModels fetches the list of available models from the Copilot API.
func GetModels(ctx context.Context, copilotToken string) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", CopilotBaseURL()+"/models", nil)
	if err != nil {
		return nil, err
	}

	headers := CopilotHeaders(copilotToken)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d body %q", resp.StatusCode, string(data))
	}

	var result ModelsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func request(ctx context.Context, method, url string, body any, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return http.DefaultClient.Do(req)
}
