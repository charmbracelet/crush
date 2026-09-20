package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
)

// ModelInfo is a single entry of the Copilot /models payload. Only the
// fields Crush needs are decoded.
type ModelInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Enabled      bool              `json:"model_picker_enabled"`
	Capabilities ModelCapabilities `json:"capabilities"`
}

// ModelCapabilities mirrors the per-model capability block.
type ModelCapabilities struct {
	Limits   ModelLimits   `json:"limits"`
	Supports ModelSupports `json:"supports"`
}

// ModelLimits mirrors the per-model usage limits.
type ModelLimits struct {
	ContextWindow int64 `json:"max_context_window_tokens"`
}

// ModelSupports mirrors the per-model feature flags.
type ModelSupports struct {
	Vision          bool     `json:"vision"`
	Thinking        bool     `json:"thinking"`
	ReasoningEffort []string `json:"reasoning_effort"`
}

type modelsResponse struct {
	Models []ModelInfo `json:"data"`
}

// modelReasoning maps the catalog's thinking flags onto the catwalk
// reasoning structure. Models that list effort levels get them all as a
// toggleable setting; models that only claim thinking support toggle
// reasoning without levels; the rest cannot reason.
func modelReasoning(s ModelSupports) catwalk.Reasoning {
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

// Models fetches the model catalog the GitHub Copilot subscription
// grants, using the OAuth token obtained from the device flow. Models
// the plan does not enable for the model picker are filtered out.
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
	// The API rejects catalog requests without the editor identity
	// headers, asking for IDE auth.
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
		return nil, &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var payload modelsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Copilot model catalog: %w", err)
	}

	models := make([]catwalk.Model, 0, len(payload.Models))
	for _, m := range payload.Models {
		if !m.Enabled {
			continue
		}
		models = append(models, catwalk.Model{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.Capabilities.Limits.ContextWindow,
			Reasoning:     modelReasoning(m.Capabilities.Supports),
			Capabilities:  catwalk.Capabilities{Vision: m.Capabilities.Supports.Vision},
		})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("the Copilot model catalog was empty")
	}
	return models, nil
}
