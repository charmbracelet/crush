package discover

import (
	"context"
	"encoding/json"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
)

func init() {
	RegisterEnricher("lmstudio", &lmstudioEnricher{})
}

// lmstudioModelsResponse mirrors the response from LM Studio's native
// GET /api/v1/models endpoint. The model array is returned under the
// "models" key (not "data" like the OpenAI-compatible endpoint). Only
// the fields we care about are decoded.
type lmstudioModelsResponse struct {
	Models []lmstudioModelEntry `json:"models"`
}

// lmstudioModelEntry is a single entry from /api/v1/models.
type lmstudioModelEntry struct {
	Key              string               `json:"key"`
	DisplayName      string               `json:"display_name"`
	MaxContextLength int64                `json:"max_context_length"`
	LoadedInstances  []lmstudioInstance   `json:"loaded_instances"`
	Capabilities     lmstudioCapabilities `json:"capabilities"`
}

// lmstudioCapabilities holds optional model capability flags from
// LM Studio's /api/v1/models endpoint.
type lmstudioCapabilities struct {
	Vision bool `json:"vision"`
}

// lmstudioInstance is a currently loaded model instance with its
// runtime config.
type lmstudioInstance struct {
	Config lmstudioInstanceConfig `json:"config"`
}

// lmstudioInstanceConfig holds per-instance runtime settings.
type lmstudioInstanceConfig struct {
	ContextLength int64 `json:"context_length"`
}

// lmstudioEnricher fetches model metadata from LM Studio's native
// /api/v1/models endpoint and populates context window, display name,
// and vision support on discovered models.
type lmstudioEnricher struct{}

func (e *lmstudioEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, stripV1Suffix(cfg.BaseURL), "/api/v1/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return models, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models, nil
	}

	var modelsResp lmstudioModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return models, nil
	}

	// Index by key for O(1) lookup.
	metaByKey := make(map[string]lmstudioModelEntry, len(modelsResp.Models))
	for _, m := range modelsResp.Models {
		metaByKey[m.Key] = m
	}

	for i := range models {
		meta, ok := metaByKey[models[i].ID]
		if !ok {
			continue
		}

		// Use the smallest known limit. A configured value may intentionally
		// restrict the model, but it cannot expand the loaded instance or the
		// model's maximum context length.
		contextWindow := models[i].ContextWindow
		contextWindow = minPositive(contextWindow, meta.MaxContextLength)
		for _, instance := range meta.LoadedInstances {
			contextWindow = minPositive(contextWindow, instance.Config.ContextLength)
		}
		models[i].ContextWindow = contextWindow

		// Display name if not already set by user.
		if models[i].Name == models[i].ID && meta.DisplayName != "" {
			models[i].Name = meta.DisplayName
		}

		// Vision support from capabilities.
		models[i].SupportsImages = meta.Capabilities.Vision
	}

	return models, nil
}

func minPositive(values ...int64) int64 {
	var result int64
	for _, value := range values {
		if value > 0 && (result == 0 || value < result) {
			result = value
		}
	}
	return result
}
