package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
)

// LiteLLMType is the provider type string users set in crush.json to
// enable automatic model discovery and enrichment from a LiteLLM proxy.
const LiteLLMType = "litellm"

func init() {
	RegisterEnricher(LiteLLMType, &litellmEnricher{})
}

type litellmEnricher struct{}

// litellmModelsResponse mirrors the LiteLLM /model/info endpoint response.
type litellmModelsResponse struct {
	Data []litellmModelInfo `json:"data"`
}

type litellmModelInfo struct {
	ModelName        string `json:"model_name"`
	ModelInfo        *litellmModelDetail `json:"model_info,omitempty"`
	ModelInfoMap     *litellmModelDetail `json:"model_info_map,omitempty"`
	LitellmParams    *litellmParams      `json:"litellm_params,omitempty"`
}

type litellmModelDetail struct {
	MaxInput  int64 `json:"max_input,omitempty"`
	MaxTokens int64 `json:"max_tokens,omitempty"`
}

type litellmParams struct {
	MaxTokens int64 `json:"max_tokens,omitempty"`
}

func (e *litellmEnricher) EnrichModels(ctx context.Context, cfg Config, models []catwalk.Model) ([]catwalk.Model, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	infoURL := baseURL + "/model/info"
	if strings.Contains(baseURL, "/v1") {
		infoURL = baseURL + "/model/info"
	} else {
		infoURL = baseURL + "/v1/model/info"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating LiteLLM info request: %w", err)
	}

	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	for k, v := range cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching LiteLLM model info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LiteLLM model info returned status %d from %s", resp.StatusCode, infoURL)
	}

	var infoResp litellmModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoResp); err != nil {
		return nil, fmt.Errorf("decoding LiteLLM model info response: %w", err)
	}

	if len(infoResp.Data) == 0 {
		slog.Warn("LiteLLM model info returned no data, using unenriched models", "endpoint", infoURL)
		return models, nil
	}

	// Build a lookup from model name to info.
	infoByName := make(map[string]litellmModelInfo, len(infoResp.Data))
	for _, info := range infoResp.Data {
		infoByName[info.ModelName] = info
	}

	var mu sync.Mutex
	enriched := make([]catwalk.Model, 0, len(models))
	var wg sync.WaitGroup

	for _, m := range models {
		model := m
		wg.Add(1)
		go func() {
			defer wg.Done()

			detail, ok := infoByName[model.ID]
			if !ok {
				// Model not in info endpoint, keep as-is.
				mu.Lock()
				enriched = append(enriched, model)
				mu.Unlock()
				return
			}

			model.ContextWindow = resolveContextWindow(detail)
			model.DefaultMaxTokens = resolveMaxTokens(detail)
			if model.Name == "" {
				model.Name = model.ID
			}

			mu.Lock()
			enriched = append(enriched, model)
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Add any models from the info endpoint that weren't in the basic list.
	for name, info := range infoByName {
		found := false
		for _, m := range models {
			if m.ID == name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		enriched = append(enriched, catwalk.Model{
			ID:               name,
			Name:             name,
			ContextWindow:    resolveContextWindow(info),
			DefaultMaxTokens: resolveMaxTokens(info),
		})
	}

	slices.SortFunc(enriched, func(a, b catwalk.Model) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})

	slog.Debug("Enriched LiteLLM models", "count", len(enriched))
	return enriched, nil
}

func resolveContextWindow(info litellmModelInfo) int64 {
	if info.ModelInfo != nil && info.ModelInfo.MaxInput > 0 {
		return info.ModelInfo.MaxInput
	}
	if info.ModelInfoMap != nil && info.ModelInfoMap.MaxInput > 0 {
		return info.ModelInfoMap.MaxInput
	}
	return 0
}

func resolveMaxTokens(info litellmModelInfo) int64 {
	if info.ModelInfo != nil && info.ModelInfo.MaxTokens > 0 {
		return info.ModelInfo.MaxTokens
	}
	if info.ModelInfoMap != nil && info.ModelInfoMap.MaxTokens > 0 {
		return info.ModelInfoMap.MaxTokens
	}
	if info.LitellmParams != nil && info.LitellmParams.MaxTokens > 0 {
		return info.LitellmParams.MaxTokens
	}
	return 0
}
