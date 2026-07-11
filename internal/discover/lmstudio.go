package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

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
	Architecture     string               `json:"architecture"`
	Type             string               `json:"type"`
	Capabilities     lmstudioCapabilities `json:"capabilities"`
	LoadedInstances  []lmstudioInstance   `json:"loaded_instances"`
}

type lmstudioCapabilities struct {
	Vision bool `json:"vision"`
}

// lmstudioInstance is a currently loaded model instance with its
// runtime config.
type lmstudioInstance struct {
	ID     string                 `json:"id"`
	Config lmstudioInstanceConfig `json:"config"`
}

// lmstudioInstanceConfig holds per-instance runtime settings.
type lmstudioInstanceConfig struct {
	ContextLength int64 `json:"context_length"`
}

// lmstudioEnricher fetches model metadata from LM Studio's native
// /api/v1/models endpoint and populates context window and display
// name on discovered models.
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
		for _, instance := range m.LoadedInstances {
			if instance.ID != "" {
				metaByKey[instance.ID] = m
			}
		}
	}

	for i := range models {
		meta, ok := metaByKey[models[i].ID]
		if !ok {
			continue
		}

		// Context window: prefer loaded instance config, fall back
		// to the model-level max.
		if models[i].ContextWindow == 0 {
			if len(meta.LoadedInstances) > 0 && meta.LoadedInstances[0].Config.ContextLength > 0 {
				models[i].ContextWindow = meta.LoadedInstances[0].Config.ContextLength
			} else if meta.MaxContextLength > 0 {
				models[i].ContextWindow = meta.MaxContextLength
			}
		}

		// Display name if not already set by user.
		if models[i].Name == models[i].ID && meta.DisplayName != "" {
			models[i].Name = meta.DisplayName
		}

		if lmstudioSupportsEnableThinking(meta) {
			models[i].CanReason = true
		}
		models[i].SupportsImages = meta.Capabilities.Vision
	}

	return models, nil
}

func lmstudioSupportsEnableThinking(meta lmstudioModelEntry) bool {
	if meta.Type != "" && meta.Type != "llm" {
		return false
	}
	architecture := strings.ToLower(meta.Architecture)
	switch architecture {
	case "qwen3", "qwen35":
		return true
	}
	id := strings.ToLower(meta.Key + " " + meta.DisplayName)
	return strings.Contains(id, "qwq") ||
		strings.Contains(id, "qwythos") ||
		strings.Contains(id, "qwen3") ||
		strings.Contains(id, "qwen-3") ||
		strings.Contains(id, "reasoning")
}
