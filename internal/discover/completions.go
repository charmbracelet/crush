package discover

import (
	"context"
	"encoding/json"
	"net/http"

	"charm.land/catwalk/v3/pkg/catwalk"
)

func init() {
	RegisterEnricher("completions", &completionsEnricher{})
}

// completionsEnricher populates context window and default max output
// tokens for OpenAI-compatible providers (type "completions", legacy
// "openai-compat") from the optional metadata some of them expose in
// their /models listing. It only fills fields that are still zero, so
// user overrides win.
//
// The base DiscoverModels call is shared by every provider type; this
// enricher runs only for completions/openai-compat providers so that
// local-server enrichers (llama.cpp, LM Studio, Ollama, LiteLLM, oMLX),
// which read the context size the model is actually loaded with, are not
// pre-empted by a larger trained maximum reported by the same endpoint.
type completionsEnricher struct{}

func (e *completionsEnricher) EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil {
		return models, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models, nil
	}

	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return models, nil
	}

	// Index the richer metadata by model ID for O(1) lookup.
	type meta struct {
		contextWindow       int64
		maxCompletionTokens *int64
	}
	metaByID := make(map[string]meta, len(modelsResp.Data))
	for _, entry := range modelsResp.Data {
		m := meta{}
		// Precedence for the context window: top-level context_window,
		// then context_length, then a nested capabilities.contextWindow
		// (e.g. 9Router).
		switch {
		case entry.ContextWindow != nil && *entry.ContextWindow > 0:
			m.contextWindow = *entry.ContextWindow
		case entry.ContextLength != nil && *entry.ContextLength > 0:
			m.contextWindow = *entry.ContextLength
		case entry.Capabilities.ContextWindow > 0:
			m.contextWindow = entry.Capabilities.ContextWindow
		}
		m.maxCompletionTokens = entry.MaxCompletionTokens
		metaByID[entry.ID] = m
	}

	// Apply metadata to discovered models, preserving existing
	// non-zero values (user overrides win).
	for i := range models {
		m, ok := metaByID[models[i].ID]
		if !ok {
			continue
		}
		if models[i].ContextWindow == 0 && m.contextWindow > 0 {
			models[i].ContextWindow = m.contextWindow
		}
		if models[i].DefaultMaxTokens == 0 {
			switch {
			case m.maxCompletionTokens != nil && *m.maxCompletionTokens > 0:
				// The provider offers a value: use it as-is.
				models[i].DefaultMaxTokens = *m.maxCompletionTokens
			case models[i].ContextWindow > 0:
				// The provider offers no output limit: fall back to 25%
				// of the context window. Sending a value equal to the full
				// window would make providers reject every request.
				models[i].DefaultMaxTokens = models[i].ContextWindow / 4
			}
		}
	}

	return models, nil
}
