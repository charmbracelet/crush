package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
)

func init() {
	RegisterEnricher("ollama", &ollamaEnricher{})
}

// ollamaShowResponse mirrors the response from Ollama's POST /api/show
// endpoint. Only the fields we care about are decoded.
type ollamaShowResponse struct {
	ModelInfo map[string]any `json:"model_info"`
}

// ollamaEnricher fetches model metadata from Ollama's /api/show
// endpoint and populates context window on discovered models.
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
	}

	results := make([]result, len(needEnrichment))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Max 5 concurrent requests.

	for ri, idx := range needEnrichment {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := doRequest(ctx, http.MethodPost, cfg.BaseURL, "/api/show",
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

			if cl := extractContextLength(showResp.ModelInfo); cl > 0 {
				results[ri] = result{index: idx, contextLength: cl}
			}
		})
	}
	wg.Wait()

	for _, r := range results {
		if r.contextLength > 0 {
			models[r.index].ContextWindow = r.contextLength
		}
	}

	return models, nil
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
