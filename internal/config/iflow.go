package config

import (
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/iflow"
)

const (
	// InferenceProviderIFlow is the identifier for the iFlow provider
	InferenceProviderIFlow catwalk.InferenceProvider = "iflow"
)

// getIFlowProvider returns the built-in iFlow provider configuration
func getIFlowProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "iFlow",
		ID:                  InferenceProviderIFlow,
		APIKey:              "$IFLOW_API_KEY",
		APIEndpoint:         "https://apis.iflow.cn/v1",
		Type:                catwalk.Type(iflow.Name),
		DefaultLargeModelID: "glm-4.7",
		DefaultSmallModelID: "glm-4.7",
		Models:              getIFlowModels(),
	}
}

// getIFlowModels returns the built-in iFlow models
func getIFlowModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "minimax-m2.1",
			Name:             "Minimax M2.1",
			ContextWindow:    204800,
			DefaultMaxTokens: 131100,
			SupportsImages:   true,
			CanReason:        true,
		},
		{
			ID:               "deepseek-v3.2",
			Name:             "DeepSeek V3.2",
			ContextWindow:    128000,
			DefaultMaxTokens: 32000,
			CanReason:        true,
		},
		{
			ID:               "glm-4.7",
			Name:             "GLM 4.7",
			ContextWindow:    200000,
			DefaultMaxTokens: 128000,
			CanReason:        true,
		},
		{
			ID:               "kimi-k2-thinking",
			Name:             "Kimi K2 Thinking",
			ContextWindow:    256000,
			DefaultMaxTokens: 32000,
			CanReason:        true,
		},
	}
}
