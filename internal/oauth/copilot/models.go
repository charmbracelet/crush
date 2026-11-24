package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
)

// modelInfo is a single entry of the Copilot /models payload. Only the
// fields Crush needs are decoded.
type modelInfo struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Enabled            bool              `json:"model_picker_enabled"`
	SupportedEndpoints []string          `json:"supported_endpoints"`
	Capabilities       modelCapabilities `json:"capabilities"`
}

type modelCapabilities struct {
	Type     string        `json:"type"`
	Limits   modelLimits   `json:"limits"`
	Supports modelSupports `json:"supports"`
}

type modelLimits struct {
	ContextWindow int64 `json:"max_context_window_tokens"`
	MaxOutput     int64 `json:"max_output_tokens"`
}

type modelSupports struct {
	Vision          bool     `json:"vision"`
	Thinking        bool     `json:"thinking"`
	ReasoningEffort []string `json:"reasoning_effort"`
}

type modelsResponse struct {
	Models []modelInfo `json:"data"`
}

// modelType picks the API a model must be called through: chat
// completions when it is served there, which is the provider default.
func modelType(endpoints []string) catwalk.Type {
	switch {
	case len(endpoints) == 0, slices.Contains(endpoints, "/chat/completions"):
		return ""
	case slices.Contains(endpoints, "/responses"):
		return catwalk.TypeResponses
	case slices.Contains(endpoints, "/v1/messages"):
		return catwalk.TypeMessages
	}
	return ""
}

// modelReasoning maps the catalog's thinking flags onto catwalk's model.
func modelReasoning(s modelSupports) catwalk.Reasoning {
	if len(s.ReasoningEffort) > 0 {
		return catwalk.Reasoning{
			Thinking:     catwalk.ThinkingToggleable,
			EffortLevels: catwalk.NewEffortLevels(s.ReasoningEffort...),
		}
	}
	if s.Thinking {
		return catwalk.Reasoning{Thinking: catwalk.ThinkingToggleable}
	}
	return catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
}

// Models fetches the models the Copilot subscription enables for the
// model picker. Crush prefers catwalk's curated entry for each ID; the
// metadata here covers models catwalk does not know yet.
func Models(ctx context.Context, token *oauth.Token) ([]catwalk.Model, error) {
	if token == nil {
		return nil, fmt.Errorf("an OAuth token is required to list GitHub Copilot models")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("OpenAI-Intent", "model-access")
	req.Header.Set("originator", "crush")
	// Without the editor identity headers the API demands IDE auth.
	for k, v := range Headers() {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Copilot model catalog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Copilot model catalog: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch Copilot model catalog: HTTP %d: %s", resp.StatusCode, body)
	}

	var payload modelsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Copilot model catalog: %w", err)
	}

	models := make([]catwalk.Model, 0, len(payload.Models))
	for _, m := range payload.Models {
		if !m.Enabled || (m.Capabilities.Type != "" && m.Capabilities.Type != "chat") {
			continue
		}
		models = append(models, catwalk.Model{
			ID:               m.ID,
			Name:             m.Name,
			Type:             modelType(m.SupportedEndpoints),
			ContextWindow:    m.Capabilities.Limits.ContextWindow,
			DefaultMaxTokens: m.Capabilities.Limits.MaxOutput,
			Reasoning:        modelReasoning(m.Capabilities.Supports),
			Capabilities:     catwalk.Capabilities{Vision: m.Capabilities.Supports.Vision},
		})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("the Copilot model catalog was empty")
	}
	return models, nil
}
