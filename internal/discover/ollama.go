package discover

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"charm.land/catwalk/v3/pkg/catwalk"
)

func init() {
	RegisterEnricher("ollama", &ollamaEnricher{})
}

// ollamaShowResponse mirrors the response from Ollama's POST /api/show
// endpoint. Only the fields we care about are decoded.
type ollamaShowResponse struct {
	ModelInfo    map[string]any `json:"model_info"`
	Capabilities []string       `json:"capabilities"`
}

// ollamaEnricher fetches model metadata from Ollama's /api/show
// endpoint and populates context window on discovered models. A model
// Ollama says cannot call tools is left out, since Crush sends tools on
// every request and Ollama refuses them for such a model.
type ollamaEnricher struct{}

func (e *ollamaEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	// Collect indices that need enrichment.
	var needEnrichment []int
	for i := range models {
		if models[i].ContextWindow == 0 {
			needEnrichment = append(needEnrichment, i)
		}
	}
	if len(needEnrichment) == 0 {
		return models, nil
	}

	// Fetch metadata concurrently with bounded parallelism.
	type result struct {
		index         int
		contextLength int64
		noTools       bool
	}

	results := make([]result, len(needEnrichment))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Max 5 concurrent requests.

	for ri, idx := range needEnrichment {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := doRequest(ctx, http.MethodPost, stripV1Suffix(cfg.BaseURL), "/api/show",
				cfg.APIKey, cfg.ExtraHeaders, resolver,
				map[string]string{"model": models[idx].ID})
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var showResp ollamaShowResponse
			if err := json.NewDecoder(resp.Body).Decode(&showResp); err != nil {
				return
			}

			results[ri] = result{
				index:         idx,
				contextLength: extractContextLength(showResp.ModelInfo),
				noTools:       lacksTools(showResp.Capabilities),
			}
		})
	}
	wg.Wait()

	var dropped []int
	for _, r := range results {
		if r.noTools {
			slog.Warn("Leaving out an Ollama model that cannot call tools",
				"provider", cfg.ID, "model", models[r.index].ID,
				"hint", "its template has no tools; Ollama's /api/show lists its capabilities")
			dropped = append(dropped, r.index)
			continue
		}
		if r.contextLength > 0 {
			models[r.index].ContextWindow = r.contextLength
		}
	}
	if len(dropped) == 0 {
		return models, nil
	}
	kept := make([]catwalk.Model, 0, len(models)-len(dropped))
	for i, m := range models {
		if !slices.Contains(dropped, i) {
			kept = append(kept, m)
		}
	}
	return kept, nil
}

// lacksTools reports whether Ollama's capabilities for a model say it
// cannot call tools. An Ollama too old to report capabilities says
// nothing, and the model is kept.
func lacksTools(capabilities []string) bool {
	return len(capabilities) > 0 && !slices.Contains(capabilities, "tools")
}

// extractContextLength finds the context_length value in Ollama's
// model_info map. The key is architecture-specific (e.g.
// "llama.context_length", "qwen2.context_length"), so we scan for any
// key ending in ".context_length".
func extractContextLength(info map[string]any) int64 {
	for k, v := range info {
		if !strings.HasSuffix(k, ".context_length") {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case json.Number:
			i, err := n.Int64()
			if err == nil {
				return i
			}
		}
	}
	return 0
}
