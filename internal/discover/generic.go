package discover

import (
	"context"
	"encoding/json"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
)

func init() {
	RegisterGenericEnricher(&genericEnricher{})
}

// genericModelsResponse captures only the fields needed for enrichment
// from the /v1/models listing. The metadata map is untyped by design —
// providers use different key names and the enricher only inspects a
// known subset.
type genericModelsResponse struct {
	Data []genericModelEntry `json:"data"`
}

type genericModelEntry struct {
	ID            string         `json:"id"`
	ContextLength int64          `json:"context_length,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// genericEnricher reads context_length and metadata.max_model_len from
// the /v1/models endpoint and populates ContextWindow on discovered
// models. It is used as a fallback for any provider without a
// provider-specific enricher.
type genericEnricher struct{}

func (e *genericEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return models, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models, nil
	}

	var modelsResp genericModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return models, nil
	}

	// Index by ID for O(1) lookup.
	metaByID := make(map[string]genericModelEntry, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		metaByID[m.ID] = m
	}

	for i := range models {
		if models[i].ContextWindow != 0 {
			continue
		}
		entry, ok := metaByID[models[i].ID]
		if !ok {
			continue
		}
		if entry.ContextLength > 0 {
			models[i].ContextWindow = entry.ContextLength
		} else if v, ok := entry.Metadata["max_model_len"]; ok {
			if f, ok := v.(float64); ok {
				models[i].ContextWindow = int64(f)
			}
		} else if v, ok := entry.Metadata["max_context_length"]; ok {
			if f, ok := v.(float64); ok {
				models[i].ContextWindow = int64(f)
			}
		}
	}

	return models, nil
}
