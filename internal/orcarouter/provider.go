// Package orcarouter contains the shared OrcaRouter provider and endpoint
// definitions used by configuration and authentication.
package orcarouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

const (
	// APIProviderID identifies the pasted API-key authentication choice.
	APIProviderID = "orcarouter"
	// OAuthProviderID identifies the OrcaRouter account-login choice.
	OAuthProviderID = "orcarouter-oauth"

	defaultAuthBaseURL = "https://www.orcarouter.ai"
	defaultAPIBaseURL  = "https://api.orcarouter.ai/v1"
	maxCatalogBytes    = 4 << 20
	maxCatalogModels   = 1000
)

// AuthBaseURL returns the authorization origin. An explicit auth origin wins
// over the shared self-hosted origin.
func AuthBaseURL() (string, error) {
	return baseURL("ORCA_AUTH_BASE_URL", defaultAuthBaseURL, false)
}

// APIBaseURL returns the OpenAI-compatible inference endpoint. An explicit
// API endpoint wins over the shared self-hosted origin.
func APIBaseURL() (string, error) {
	return baseURL("ORCA_API_BASE_URL", defaultAPIBaseURL, true)
}

func baseURL(explicitEnv, fallback string, appendV1 bool) (string, error) {
	raw := os.Getenv(explicitEnv)
	if raw == "" {
		raw = os.Getenv("ORCA_BASE_URL")
	}
	if raw == "" {
		raw = fallback
	}
	raw = strings.TrimRight(raw, "/")

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid OrcaRouter base URL in %s", explicitEnv)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid OrcaRouter base URL in %s", explicitEnv)
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("OrcaRouter base URL in %s must use HTTPS unless it is loopback", explicitEnv)
	}
	if appendV1 && !strings.HasSuffix(u.Path, "/v1") {
		u.Path = strings.TrimRight(u.Path, "/") + "/v1"
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// Providers returns the two explicit authentication choices with a verified
// offline model seed.
func Providers() ([]catwalk.Provider, error) {
	apiBase, err := APIBaseURL()
	if err != nil {
		return nil, err
	}
	return providersWithModels(apiBase, fallbackModels()), nil
}

func providersWithModels(apiBase string, models []catwalk.Model) []catwalk.Provider {
	return []catwalk.Provider{
		{
			Name:                "OrcaRouter - API",
			ID:                  APIProviderID,
			APIKey:              "$ORCAROUTER_API_KEY",
			APIEndpoint:         apiBase,
			Type:                catwalk.TypeOpenAICompat,
			DefaultLargeModelID: "openai/gpt-5.5",
			DefaultSmallModelID: "google/gemini-3.5-flash",
			Models:              slices.Clone(models),
		},
		{
			Name:                "OrcaRouter - Auth",
			ID:                  OAuthProviderID,
			APIEndpoint:         apiBase,
			Type:                catwalk.TypeOpenAICompat,
			DefaultLargeModelID: "openai/gpt-5.5",
			DefaultSmallModelID: "google/gemini-3.5-flash",
			Models:              slices.Clone(models),
		},
	}
}

// FetchProviders refreshes both authentication choices from OrcaRouter's
// public OpenAI-compatible model catalog.
func FetchProviders(ctx context.Context) ([]catwalk.Provider, error) {
	apiBase, err := APIBaseURL()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create OrcaRouter catalog request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OrcaRouter catalog: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch OrcaRouter catalog: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read OrcaRouter catalog: %w", err)
	}
	if len(body) > maxCatalogBytes {
		return nil, fmt.Errorf("read OrcaRouter catalog: response exceeds %d bytes", maxCatalogBytes)
	}

	var response catalogResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode OrcaRouter catalog: %w", err)
	}
	if len(response.Data) > maxCatalogModels {
		return nil, fmt.Errorf("decode OrcaRouter catalog: response exceeds %d models", maxCatalogModels)
	}

	seeds := make(map[string]catwalk.Model)
	for _, model := range fallbackModels() {
		seeds[model.ID] = model
	}

	models := make([]catwalk.Model, 0, len(response.Data))
	seen := make(map[string]struct{}, len(response.Data))
	for _, item := range response.Data {
		if !validCatalogItem(item) {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}

		model := seeds[item.ID]
		model.ID = item.ID
		model.Name = item.Name
		if model.Name == "" {
			model.Name = item.ID
		}
		if item.ContextLength > 0 {
			model.ContextWindow = item.ContextLength
		} else if item.TopProvider.ContextLength > 0 {
			model.ContextWindow = item.TopProvider.ContextLength
		}
		if item.MaxCompletionTokens > 0 {
			model.DefaultMaxTokens = item.MaxCompletionTokens
		} else if item.TopProvider.MaxCompletionTokens > 0 {
			model.DefaultMaxTokens = item.TopProvider.MaxCompletionTokens
		}
		model.SupportsImages = model.SupportsImages || slices.Contains(item.Architecture.InputModalities, "image")
		model.CostPer1MIn = parsePrice(item.Pricing.PromptPerMillion, item.Pricing.Prompt)
		model.CostPer1MOut = parsePrice(item.Pricing.CompletionPerMillion, item.Pricing.Completion)
		models = append(models, model)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("decode OrcaRouter catalog: no OpenAI-compatible models")
	}

	return providersWithModels(apiBase, models), nil
}

type catalogResponse struct {
	Data []catalogItem `json:"data"`
}

type catalogItem struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Object                 string   `json:"object"`
	ContextLength          int64    `json:"context_length"`
	MaxCompletionTokens    int64    `json:"max_completion_tokens"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types"`
	Architecture           struct {
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	TopProvider struct {
		ContextLength       int64 `json:"context_length"`
		MaxCompletionTokens int64 `json:"max_completion_tokens"`
	} `json:"top_provider"`
	Pricing struct {
		Prompt               string `json:"prompt"`
		Completion           string `json:"completion"`
		PromptPerMillion     string `json:"prompt_per_million"`
		CompletionPerMillion string `json:"completion_per_million"`
	} `json:"pricing"`
}

func validCatalogItem(item catalogItem) bool {
	return item.ID != "" && len(item.ID) <= 256 &&
		(item.Object == "" || item.Object == "model") &&
		slices.Contains(item.SupportedEndpointTypes, "openai")
}

func parsePrice(perMillion, perToken string) float64 {
	if value, err := strconv.ParseFloat(perMillion, 64); err == nil {
		return value
	}
	value, err := strconv.ParseFloat(perToken, 64)
	if err != nil {
		return 0
	}
	return value * 1_000_000
}

func fallbackModels() []catwalk.Model {
	reasoningLevels := []string{"low", "medium", "high", "xhigh"}
	return []catwalk.Model{
		{
			ID:                     "openai/gpt-5.5",
			Name:                   "OpenAI: GPT-5.5",
			ContextWindow:          1_048_576,
			DefaultMaxTokens:       128_000,
			CanReason:              true,
			ReasoningLevels:        slices.Clone(reasoningLevels),
			DefaultReasoningEffort: "medium",
			SupportsImages:         true,
		},
		{
			ID:                     "anthropic/claude-opus-4.8",
			Name:                   "Anthropic: Claude Opus 4.8",
			ContextWindow:          1_000_000,
			DefaultMaxTokens:       128_000,
			CanReason:              true,
			ReasoningLevels:        slices.Clone(reasoningLevels),
			DefaultReasoningEffort: "medium",
			SupportsImages:         true,
		},
		{
			ID:                     "google/gemini-3.5-flash",
			Name:                   "Google: Gemini 3.5 Flash",
			ContextWindow:          1_048_576,
			DefaultMaxTokens:       65_536,
			CanReason:              true,
			ReasoningLevels:        slices.Clone(reasoningLevels),
			DefaultReasoningEffort: "medium",
			SupportsImages:         true,
		},
		{
			ID:                     "deepseek/deepseek-v4-pro",
			Name:                   "DeepSeek: DeepSeek V4 Pro",
			ContextWindow:          1_048_576,
			DefaultMaxTokens:       384_000,
			CanReason:              true,
			ReasoningLevels:        slices.Clone(reasoningLevels),
			DefaultReasoningEffort: "medium",
		},
		{
			ID:                     "orcarouter/auto",
			Name:                   "OrcaRouter Auto",
			ContextWindow:          1_000_000,
			DefaultMaxTokens:       64_000,
			CanReason:              true,
			ReasoningLevels:        slices.Clone(reasoningLevels),
			DefaultReasoningEffort: "medium",
			SupportsImages:         true,
		},
	}
}
