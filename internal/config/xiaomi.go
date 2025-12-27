package config

import (
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/xiaomi"
)

const (
	// InferenceProviderXiaomi is the identifier for the Xiaomi provider
	InferenceProviderXiaomi catwalk.InferenceProvider = "xiaomi"
)

// getXiaomiProvider returns the built-in Xiaomi provider configuration
func getXiaomiProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "Xiaomi",
		ID:                  InferenceProviderXiaomi,
		APIKey:              "$XIAOMI_API_KEY",
		APIEndpoint:         "https://api.xiaomimimo.com/v1",
		Type:                catwalk.Type(xiaomi.Name),
		DefaultLargeModelID: "mimo-v2-flash",
		DefaultSmallModelID: "mimo-v2-flash",
		Models:              getXiaomiModels(),
	}
}

// getXiaomiModels returns the built-in Xiaomi models
func getXiaomiModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "mimo-v2-flash",
			Name:             "Mimo V2 Flash",
			ContextWindow:    256000,
			DefaultMaxTokens: 128000,
			CanReason:        true,
			// Reasoning levels and default effort are handled via ModelCfg in agent
		},
	}
}
