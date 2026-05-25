package proto

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
)

// ConfigSetRequest represents a request to set a config field.
type ConfigSetRequest struct {
	Scope config.Scope `json:"scope"`
	Key   string       `json:"key"`
	Value any          `json:"value"`
}

// ConfigRemoveRequest represents a request to remove a config field.
type ConfigRemoveRequest struct {
	Scope config.Scope `json:"scope"`
	Key   string       `json:"key"`
}

// ConfigHasFieldResponse represents whether a config field exists.
type ConfigHasFieldResponse struct {
	Exists bool `json:"exists"`
}

// ConfigModelRequest represents a request to update the preferred model.
type ConfigModelRequest struct {
	Scope     config.Scope             `json:"scope"`
	ModelType config.SelectedModelType `json:"model_type"`
	Model     config.SelectedModel     `json:"model"`
}

// ConfigCompactRequest represents a request to set compact mode.
type ConfigCompactRequest struct {
	Scope   config.Scope `json:"scope"`
	Enabled bool         `json:"enabled"`
}

// APIKeyKind discriminates the kind of credential carried in a
// ConfigProviderKeyRequest. JSON's `any` loses Go type information, so
// the wire format names the kind explicitly and the server decodes
// APIKey accordingly.
type APIKeyKind string

const (
	// APIKeyKindString is a plain string API key.
	APIKeyKindString APIKeyKind = "string"
	// APIKeyKindOAuth is an oauth.Token credential.
	APIKeyKindOAuth APIKeyKind = "oauth"
)

// ConfigProviderKeyRequest represents a request to set a provider API
// key. APIKey is the raw JSON for the credential; Kind selects the
// concrete Go type APIKey should be decoded into via DecodeAPIKey.
type ConfigProviderKeyRequest struct {
	Scope      config.Scope    `json:"scope"`
	ProviderID string          `json:"provider_id"`
	Kind       APIKeyKind      `json:"kind"`
	APIKey     json.RawMessage `json:"api_key"`
}

// DecodeAPIKey decodes APIKey into the Go type indicated by Kind. It
// returns a string for APIKeyKindString and a *oauth.Token for
// APIKeyKindOAuth. An unknown kind or malformed payload is reported
// as an error.
func (r ConfigProviderKeyRequest) DecodeAPIKey() (any, error) {
	switch r.Kind {
	case APIKeyKindString:
		var s string
		if err := json.Unmarshal(r.APIKey, &s); err != nil {
			return nil, fmt.Errorf("decode api key string: %w", err)
		}
		return s, nil
	case APIKeyKindOAuth:
		var tok oauth.Token
		if err := json.Unmarshal(r.APIKey, &tok); err != nil {
			return nil, fmt.Errorf("decode api key oauth token: %w", err)
		}
		return &tok, nil
	default:
		return nil, fmt.Errorf("unsupported api key kind %q", r.Kind)
	}
}

// ConfigRefreshOAuthRequest represents a request to refresh an OAuth token.
type ConfigRefreshOAuthRequest struct {
	Scope      config.Scope `json:"scope"`
	ProviderID string       `json:"provider_id"`
}

// ImportCopilotResponse represents the response from importing Copilot credentials.
type ImportCopilotResponse struct {
	Token   any  `json:"token"`
	Success bool `json:"success"`
}

// ProjectNeedsInitResponse represents whether a project needs initialization.
type ProjectNeedsInitResponse struct {
	NeedsInit bool `json:"needs_init"`
}

// ProjectInitPromptResponse represents the project initialization prompt.
type ProjectInitPromptResponse struct {
	Prompt string `json:"prompt"`
}

// LSPStartRequest represents a request to start an LSP for a path.
type LSPStartRequest struct {
	Path string `json:"path"`
}

// FileTrackerReadRequest represents a request to record a file read.
type FileTrackerReadRequest struct {
	SessionID string `json:"session_id"`
	Path      string `json:"path"`
}

// MCPNameRequest represents a request targeting a named MCP server.
type MCPNameRequest struct {
	Name string `json:"name"`
}

// MCPReadResourceRequest represents a request to read an MCP resource.
type MCPReadResourceRequest struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// MCPGetPromptRequest represents a request to get an MCP prompt.
type MCPGetPromptRequest struct {
	ClientID string            `json:"client_id"`
	PromptID string            `json:"prompt_id"`
	Args     map[string]string `json:"args"`
}

// MCPGetPromptResponse represents the response from getting an MCP prompt.
type MCPGetPromptResponse struct {
	Prompt string `json:"prompt"`
}
