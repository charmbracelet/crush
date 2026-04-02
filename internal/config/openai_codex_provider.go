package config

import (
	"slices"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
)

const openAICodexBaseURL = "https://chatgpt.com/backend-api/codex"

var openAICodexPinnedModels = []string{
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-5.3-codex",
	"gpt-5.2-codex",
}

func ensureOpenAICodexProvider(providers []catwalk.Provider) []catwalk.Provider {
	id := catwalk.InferenceProvider(openai_codex.ProviderID)
	idx := slices.IndexFunc(providers, func(p catwalk.Provider) bool { return p.ID == id })
	if idx >= 0 {
		existing := providers[idx]
		if existing.DefaultLargeModelID == "" {
			existing.DefaultLargeModelID = openAICodexPinnedModels[0]
		}
		if existing.DefaultSmallModelID == "" {
			existing.DefaultSmallModelID = openAICodexPinnedModels[1]
		}
		if len(existing.Models) == 0 {
			existing.Models = buildPinnedOpenAICodexModels(providers)
		}
		providers[idx] = existing
		return providers
	}

	return append(providers, catwalk.Provider{
		Name:                openai_codex.ProviderName,
		ID:                  id,
		APIEndpoint:         openAICodexBaseURL,
		Type:                catwalk.TypeOpenAI,
		DefaultLargeModelID: openAICodexPinnedModels[0],
		DefaultSmallModelID: openAICodexPinnedModels[1],
		DefaultHeaders: map[string]string{
			"originator":  "crush",
			"OpenAI-Beta": "responses=experimental",
		},
		Models: buildPinnedOpenAICodexModels(providers),
	})
}

func buildPinnedOpenAICodexModels(providers []catwalk.Provider) []catwalk.Model {
	byID := make(map[string]catwalk.Model)
	for _, provider := range providers {
		for _, model := range provider.Models {
			if model.ID != "" {
				if _, exists := byID[model.ID]; !exists {
					byID[model.ID] = model
				}
			}
		}
	}

	models := make([]catwalk.Model, 0, len(openAICodexPinnedModels))
	for _, id := range openAICodexPinnedModels {
		if model, ok := byID[id]; ok {
			models = append(models, model)
			continue
		}
		models = append(models, catwalk.Model{
			ID:                     id,
			Name:                   id,
			ContextWindow:          200000,
			DefaultMaxTokens:       32768,
			CanReason:              strings.HasPrefix(id, "gpt-5"),
			DefaultReasoningEffort: "medium",
		})
	}
	return models
}
