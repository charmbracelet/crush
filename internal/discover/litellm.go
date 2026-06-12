package discover

import (
	"context"
	"encoding/json"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
)

// litellmModelInfoResponse mirrors the response from LiteLLM's
// /model/info endpoint, which returns rich metadata including context
// windows, max tokens, and pricing.
type litellmModelInfoResponse struct {
	Data []litellmModelInfo `json:"data"`
}

// litellmModelInfo is a single entry from /model/info.
type litellmModelInfo struct {
	ModelName string           `json:"model_name"`
	ModelInfo litellmModelMeta `json:"model_info"`
}

// litellmModelMeta holds the metadata fields we care about from
// LiteLLM's model_info block.
type litellmModelMeta struct {
	MaxInputTokens     *int64   `json:"max_input_tokens"`
	MaxOutputTokens    *int64   `json:"max_output_tokens"`
	InputCostPerToken  *float64 `json:"input_cost_per_token"`
	OutputCostPerToken *float64 `json:"output_cost_per_token"`
	Mode               string   `json:"mode"`
}

func init() {
	RegisterEnricher("litellm", &litellmEnricher{})
}

// litellmEnricher fetches model metadata from LiteLLM's /model/info
// endpoint and populates context window, max tokens, and pricing on
// discovered models.
type litellmEnricher struct{}

func (e *litellmEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/model/info", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return models, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models, nil
	}

	var infoResp litellmModelInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoResp); err != nil {
		return models, nil
	}

	// Index metadata by model name for O(1) lookup.
	metaByID := make(map[string]litellmModelMeta, len(infoResp.Data))
	for _, entry := range infoResp.Data {
		metaByID[entry.ModelName] = entry.ModelInfo
	}

	// Apply metadata to discovered models, preserving existing
	// non-zero values (user overrides win).
	for i := range models {
		meta, ok := metaByID[models[i].ID]
		if !ok {
			continue
		}
		if models[i].ContextWindow == 0 && meta.MaxInputTokens != nil {
			models[i].ContextWindow = *meta.MaxInputTokens
		}
		if models[i].DefaultMaxTokens == 0 && meta.MaxOutputTokens != nil {
			models[i].DefaultMaxTokens = *meta.MaxOutputTokens
		}
		if models[i].CostPer1MIn == 0 && meta.InputCostPerToken != nil {
			models[i].CostPer1MIn = *meta.InputCostPerToken * 1_000_000
		}
		if models[i].CostPer1MOut == 0 && meta.OutputCostPerToken != nil {
			models[i].CostPer1MOut = *meta.OutputCostPerToken * 1_000_000
		}
	}

	return models, nil
}
