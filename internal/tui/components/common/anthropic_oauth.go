package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/lacymorrow/lash/internal/auth"
	"github.com/lacymorrow/lash/internal/config"
)

// AnthropicOAuthHandler handles the OAuth flow for both anthropic and anthropic-max providers
type AnthropicOAuthHandler struct {
	ProviderID string
	ModelID    string
	ModelType  config.SelectedModelType
}

// StartOAuth initiates the OAuth flow and returns the authorization URL and verifier
func (h *AnthropicOAuthHandler) StartOAuth() (url string, verifier string, err error) {
	// Use "max" mode for anthropic-max (Claude Code), "console" for regular anthropic
	mode := "console"
	if h.ProviderID == "anthropic-max" {
		mode = "max"
	}
	return auth.AuthorizeURL(mode)
}

// ExchangeCode exchanges the authorization code for tokens and configures the provider
func (h *AnthropicOAuthHandler) ExchangeCode(code string, verifier string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx

	// Exchange the code for tokens
	info, err := auth.ExchangeCode(code, verifier)
	if err != nil {
		return fmt.Errorf("OAuth exchange failed: %w", err)
	}

	// Persist the OAuth tokens
	if err := auth.Set("anthropic", info); err != nil {
		return fmt.Errorf("failed to persist OAuth tokens: %w", err)
	}

	accessToken := info.Access
	finalAPIValue := ""

	// For anthropic-max (Claude Code/Max), use OAuth token directly
	// For regular anthropic, create an API key
	if h.ProviderID == "anthropic-max" {
		// Claude Code/Max uses OAuth token directly
		finalAPIValue = "Bearer " + accessToken
	} else {
		// Regular Anthropic Console - create API key
		apiKey := h.createAPIKey(accessToken)
		if apiKey != "" {
			finalAPIValue = apiKey
		} else {
			// Fallback to Bearer token if API key creation failed
			finalAPIValue = "Bearer " + accessToken
		}
	}

	// Configure the provider
	return h.configureProvider(finalAPIValue)
}

// createAPIKey attempts to create an API key using the OAuth access token
func (h *AnthropicOAuthHandler) createAPIKey(accessToken string) string {
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/api/oauth/claude_cli/create_api_key", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil || resp == nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	var out struct {
		RawKey string `json:"raw_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ""
	}
	return out.RawKey
}

// configureProvider saves the provider configuration with the API key or OAuth token
func (h *AnthropicOAuthHandler) configureProvider(apiValue string) error {
	cfg := config.Get()
	pc, ok := cfg.Providers.Get(h.ProviderID)
	
	if !ok {
		// Create new provider config
		pc = h.createProviderConfig()
	} else {
		// Ensure models are populated if needed
		if len(pc.Models) == 0 {
			h.populateModels(&pc)
		}
		// Ensure BaseURL is set and doesn't contain environment variables
		if pc.BaseURL == "" || strings.Contains(pc.BaseURL, "$") {
			pc.BaseURL = config.DefaultAnthropicBaseURL
		}
		// Ensure Type is set
		if pc.Type == "" {
			pc.Type = catwalk.TypeAnthropic
		}
	}

	// Ensure ExtraHeaders is initialized
	if pc.ExtraHeaders == nil {
		pc.ExtraHeaders = map[string]string{}
	}

	// Set the API key or OAuth token
	pc.APIKey = apiValue
	pc.Disable = false
	pc.ExtraHeaders[config.HeaderAnthropicVersion] = config.DefaultAnthropicAPIVer

	// Save the configuration
	cfg.Providers.Set(h.ProviderID, pc)
	return cfg.SetConfigField("providers."+h.ProviderID, pc)
}

// createProviderConfig creates a new provider configuration
func (h *AnthropicOAuthHandler) createProviderConfig() config.ProviderConfig {
	// For anthropic-max, copy from the base anthropic provider
	searchID := h.ProviderID
	if h.ProviderID == "anthropic-max" {
		searchID = string(catwalk.InferenceProviderAnthropic)
	}

	// Always use the default Anthropic base URL - don't copy APIEndpoint
	// which might contain environment variables like $ANTHROPIC_API_ENDPOINT
	baseURL := config.DefaultAnthropicBaseURL

	known, _ := config.Providers()
	for _, kp := range known {
		if string(kp.ID) == searchID {
			return config.ProviderConfig{
				ID:           h.ProviderID,
				Name:         kp.Name,
				BaseURL:      baseURL, // Always use the resolved URL
				Type:         kp.Type,
				ExtraHeaders: map[string]string{},
				Models:       kp.Models,
			}
		}
	}

	// Fallback if provider not found
	return config.ProviderConfig{
		ID:           h.ProviderID,
		Name:         "Anthropic",
		Type:         catwalk.TypeAnthropic,
		BaseURL:      baseURL,
		ExtraHeaders: map[string]string{},
	}
}

// populateModels ensures the provider config has models populated
func (h *AnthropicOAuthHandler) populateModels(pc *config.ProviderConfig) {
	searchID := h.ProviderID
	if h.ProviderID == "anthropic-max" {
		searchID = string(catwalk.InferenceProviderAnthropic)
	}

	known, _ := config.Providers()
	for _, kp := range known {
		if string(kp.ID) == searchID {
			pc.Models = kp.Models
			// Don't copy APIEndpoint as it might contain env vars
			// Always ensure we have the correct base URL
			if pc.BaseURL == "" || strings.Contains(pc.BaseURL, "$") {
				pc.BaseURL = config.DefaultAnthropicBaseURL
			}
			if pc.Type == "" {
				pc.Type = kp.Type
			}
			break
		}
	}
	
	// Final fallback to ensure BaseURL and Type are never empty or contain env vars
	if pc.BaseURL == "" || strings.Contains(pc.BaseURL, "$") {
		pc.BaseURL = config.DefaultAnthropicBaseURL
	}
	if pc.Type == "" {
		pc.Type = catwalk.TypeAnthropic
	}
}

// GetTitleText returns the appropriate title for the OAuth dialog/screen
func (h *AnthropicOAuthHandler) GetTitleText() string {
	if h.ProviderID == "anthropic-max" {
		return "Sign in to Claude Pro/Max"
	}
	return "Sign in to Anthropic Console"
}

// GetBrowserMessage returns the appropriate browser message
func (h *AnthropicOAuthHandler) GetBrowserMessage() string {
	if h.ProviderID == "anthropic-max" {
		return "Opened browser for Claude Pro/Max sign-in"
	}
	return "Opened browser for Anthropic Console sign-in"
}