package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	xetag "github.com/charmbracelet/x/etag"
)

var _ liveProviderClient = (*realCopilotModelsClient)(nil)

type copilotTokenRefresher func(context.Context, string) (*oauth.Token, error)

type realCopilotModelsClient struct {
	baseURL      string
	apiKey       string
	oauthToken   *oauth.Token
	refreshToken copilotTokenRefresher
}

func (r *realCopilotModelsClient) Get(ctx context.Context, etag string) (catwalk.Provider, error) {
	var result catwalk.Provider
	baseURL := strings.TrimRight(r.baseURL, "/")
	accessToken, err := r.accessToken(ctx)
	if err != nil {
		return result, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return result, fmt.Errorf("could not create request: %w", err)
	}
	xetag.Request(req, etag)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	for key, value := range copilot.Headers() {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotModified {
		return result, catwalk.ErrNotModified
	}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var models copilotModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return result, fmt.Errorf("failed to decode response: %w", err)
	}

	result = catwalk.Provider{
		ID:          catwalk.InferenceProviderCopilot,
		APIEndpoint: baseURL,
		Type:        catwalk.TypeOpenAICompat,
		Models:      copilotModelsToCatwalkModels(models),
	}
	return result, nil
}

func (r *realCopilotModelsClient) accessToken(ctx context.Context) (string, error) {
	if r.oauthToken != nil {
		if token := strings.TrimSpace(r.oauthToken.AccessToken); token != "" && !r.oauthToken.IsExpired() {
			r.apiKey = token
			return token, nil
		}

		refreshToken := strings.TrimSpace(r.oauthToken.RefreshToken)
		if refreshToken != "" {
			refresh := r.refreshToken
			if refresh == nil {
				refresh = copilot.RefreshToken
			}
			refreshedToken, err := refresh(ctx, refreshToken)
			if err != nil {
				return "", fmt.Errorf("failed to refresh Copilot token: %w", err)
			}
			r.oauthToken = refreshedToken
			r.apiKey = strings.TrimSpace(refreshedToken.AccessToken)
			if r.apiKey != "" {
				return r.apiKey, nil
			}
		}
	}

	if token := strings.TrimSpace(r.apiKey); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("missing Copilot access token")
}

type copilotModelsResponse struct {
	Data []copilotModel `json:"data"`
}

type copilotModel struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Version      string                   `json:"version"`
	Capabilities copilotModelCapabilities `json:"capabilities"`
}

type copilotModelCapabilities struct {
	Limits   copilotModelLimits   `json:"limits"`
	Supports copilotModelSupports `json:"supports"`
}

type copilotModelLimits struct {
	MaxContextWindowTokens int64 `json:"max_context_window_tokens"`
	MaxOutputTokens        int64 `json:"max_output_tokens"`
}

type copilotModelSupports struct {
	Vision           bool     `json:"vision"`
	ReasoningEffort  []string `json:"reasoning_effort"`
	AdaptiveThinking bool     `json:"adaptive_thinking"`
}

var copilotVersionedModelRegexp = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

func copilotModelsToCatwalkModels(response copilotModelsResponse) []catwalk.Model {
	aliasedVersions := make(map[string]bool, len(response.Data))
	for _, model := range response.Data {
		if model.Version != "" && model.ID != model.Version {
			aliasedVersions[model.Version] = true
		}
	}

	seen := make(map[string]bool, len(response.Data))
	models := make([]catwalk.Model, 0, len(response.Data))
	for _, model := range response.Data {
		if shouldSkipCopilotModel(model, aliasedVersions) || seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		reasoningLevels := model.Capabilities.Supports.ReasoningEffort
		models = append(models, catwalk.Model{
			ID:               model.ID,
			Name:             model.Name,
			ContextWindow:    model.Capabilities.Limits.MaxContextWindowTokens,
			DefaultMaxTokens: model.Capabilities.Limits.MaxOutputTokens,
			CanReason:        model.Capabilities.Supports.AdaptiveThinking || len(reasoningLevels) > 0,
			ReasoningLevels:  reasoningLevels,
			SupportsImages:   model.Capabilities.Supports.Vision,
		})
	}

	slices.SortStableFunc(models, func(a, b catwalk.Model) int {
		return strings.Compare(a.ID, b.ID)
	})
	return models
}

func shouldSkipCopilotModel(model copilotModel, aliasedVersions map[string]bool) bool {
	return model.ID == "" ||
		aliasedVersions[model.ID] ||
		copilotVersionedModelRegexp.MatchString(model.ID) ||
		strings.Contains(model.ID, "embedding") ||
		strings.HasPrefix(model.ID, "accounts/msft/routers") ||
		strings.HasPrefix(model.ID, "oswe-vscode") ||
		strings.HasPrefix(model.ID, "lark") ||
		strings.HasPrefix(model.ID, "mai-code") ||
		model.ID == "gpt-4-o-preview" ||
		model.ID == "trajectory-compaction"
}
