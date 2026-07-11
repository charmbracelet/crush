package discover

import (
	"context"
	"encoding/json"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
)

func init() {
	RegisterEnricher("omlx", &omlxEnricher{})
}

// omlxModelsStatusResponse mirrors the response from oMLX's
// GET /models/status endpoint, which returns model metadata
// including max_context_window and max_tokens.
type omlxModelsStatusResponse struct {
	Models []omlxModelStatus `json:"models"`
}

// omlxModelStatus is a single entry from /v1/models/status.
type omlxModelStatus struct {
	ID               string `json:"id"`
	MaxContextWindow *int64 `json:"max_context_window"`
	MaxTokens        *int64 `json:"max_tokens"`
}

// omlxEnricher fetches model metadata from oMLX's /models/status
// endpoint and populates context window and max tokens on discovered
// models.
type omlxEnricher struct{}

func (e *omlxEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	// oMLX serves /models/status under the OpenAI-compatible /v1
	// namespace, so the path is relative to the configured base URL
	// (which already includes /v1) rather than the server root.
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models/status", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return models, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models, nil
	}

	var statusResp omlxModelsStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return models, nil
	}

	// Index by model ID for O(1) lookup.
	metaByID := make(map[string]omlxModelStatus, len(statusResp.Models))
	for _, m := range statusResp.Models {
		metaByID[m.ID] = m
	}

	for i := range models {
		meta, ok := metaByID[models[i].ID]
		if !ok {
			continue
		}
		if models[i].ContextWindow == 0 && meta.MaxContextWindow != nil {
			models[i].ContextWindow = *meta.MaxContextWindow
		}
		if models[i].DefaultMaxTokens == 0 && meta.MaxTokens != nil {
			models[i].DefaultMaxTokens = *meta.MaxTokens
		}
	}

	return models, nil
}
