package discover

import (
	"context"
	"encoding/json"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
)

func init() {
	RegisterEnricher("llamaswap", &llamaswapEnricher{})
}

// llamaswapModelsResponse mirrors the response from llama-swaps's
// GET /models endpoint.
type llamaswapModelsResponse struct {
	Data []llamaswapModelEntry `json:"data"`
}

// llamaswapModelEntry is a single entry from /models.
type llamaswapModelEntry struct {
	ID   string                 `json:"id"`
	Meta llamaswapModelMetadata `json:"meta"`
}

// llamaswapModelMetadata is the provider-specific metadata for a model.
type llamaswapModelMetadata struct {
	// Since llama-swap allows the user to supply any values they want to be
	// included in the metadata for a model in its config, we don't define our
	// own llamaswapUserDefinedProperties type like other enrichers do.
	// Instead we support configuring *any* property that a user could
	// configure for a model in crush.json by using catwalk.Model directly.
	LlamaSwap catwalk.Model `json:"llamaswap"`
}

// llamaswapEnricher fetches user-defined model metadata from llama-swap's
// /models endpoint and populates the context window, display name,
// and other select supported properties that the user may define in the
// llama-swap "metadata" on discovered models.
type llamaswapEnricher struct{}

func (e *llamaswapEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return models, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models, nil
	}

	var modelsResp llamaswapModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return models, nil
	}

	// Index the model data by ID for quick lookup.
	metaByID := make(map[string]catwalk.Model, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		metaByID[m.ID] = m.Meta.LlamaSwap
	}

	for i := range models {
		meta, ok := metaByID[models[i].ID]
		if !ok {
			continue
		}

		if models[i].Name == models[i].ID && meta.Name != "" {
			models[i].Name = meta.Name
		}
		if models[i].ContextWindow == 0 && meta.ContextWindow != 0 {
			models[i].ContextWindow = meta.ContextWindow
		}
		if models[i].DefaultMaxTokens == 0 && meta.DefaultMaxTokens != 0 {
			models[i].DefaultMaxTokens = meta.DefaultMaxTokens
		}
		if models[i].CostPer1MIn == 0 && meta.CostPer1MIn != 0 {
			models[i].CostPer1MIn = meta.CostPer1MIn
		}
		if models[i].CostPer1MOut == 0 && meta.CostPer1MOut != 0 {
			models[i].CostPer1MOut = meta.CostPer1MOut
		}
		if models[i].CostPer1MInCached == 0 && meta.CostPer1MInCached != 0 {
			models[i].CostPer1MInCached = meta.CostPer1MInCached
		}
		if models[i].CostPer1MOutCached == 0 && meta.CostPer1MOutCached != 0 {
			models[i].CostPer1MOutCached = meta.CostPer1MOutCached
		}
		if len(models[i].ReasoningLevels) == 0 {
			models[i].ReasoningLevels = meta.ReasoningLevels
		}
	}

	return models, nil
}
