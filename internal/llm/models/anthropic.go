package models

const (
	ProviderAnthropic ModelProvider = "anthropic"

	// Models
	Claude35Sonnet ModelID = "claude-3.5-sonnet"
	Claude3Haiku   ModelID = "claude-3-haiku"
	Claude37Sonnet ModelID = "claude-3.7-sonnet"
	Claude35Haiku  ModelID = "claude-3.5-haiku"
	Claude3Opus    ModelID = "claude-3-opus"
	Claude4Opus    ModelID = "claude-4-opus"
	Claude4Sonnet  ModelID = "claude-4-sonnet"
)

// https://docs.anthropic.com/en/docs/about-claude/models/all-models
var AnthropicModels = map[ModelID]Model{
	Claude35Sonnet: {
		ID:                  Claude35Sonnet,
		Name:                "Claude 3.5 Sonnet",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-3-5-sonnet-latest",
		CostPer1MIn:         3.0,
		CostPer1MInCached:   3.75,
		CostPer1MOutCached:  0.30,
		CostPer1MOut:        15.0,
		ContextWindow:       200000,
		DefaultMaxTokens:    5000,
		SupportsAttachments: true,
	},
	Claude3Haiku: {
		ID:                  Claude3Haiku,
		Name:                "Claude 3 Haiku",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-3-haiku-20240307", // doesn't support "-latest"
		CostPer1MIn:         0.25,
		CostPer1MInCached:   0.30,
		CostPer1MOutCached:  0.03,
		CostPer1MOut:        1.25,
		ContextWindow:       200000,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
	Claude37Sonnet: {
		ID:                  Claude37Sonnet,
		Name:                "Claude 3.7 Sonnet",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-3-7-sonnet-latest",
		CostPer1MIn:         3.0,
		CostPer1MInCached:   3.75,
		CostPer1MOutCached:  0.30,
		CostPer1MOut:        15.0,
		ContextWindow:       200000,
		DefaultMaxTokens:    50000,
		CanReason:           true,
		SupportsAttachments: true,
	},
	Claude35Haiku: {
		ID:                  Claude35Haiku,
		Name:                "Claude 3.5 Haiku",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-3-5-haiku-latest",
		CostPer1MIn:         0.80,
		CostPer1MInCached:   1.0,
		CostPer1MOutCached:  0.08,
		CostPer1MOut:        4.0,
		ContextWindow:       200000,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
	Claude3Opus: {
		ID:                  Claude3Opus,
		Name:                "Claude 3 Opus",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-3-opus-latest",
		CostPer1MIn:         15.0,
		CostPer1MInCached:   18.75,
		CostPer1MOutCached:  1.50,
		CostPer1MOut:        75.0,
		ContextWindow:       200000,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
	Claude4Sonnet: {
		ID:                  Claude4Sonnet,
		Name:                "Claude 4 Sonnet",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-sonnet-4-20250514",
		CostPer1MIn:         3.0,
		CostPer1MInCached:   3.75,
		CostPer1MOutCached:  0.30,
		CostPer1MOut:        15.0,
		ContextWindow:       200000,
		DefaultMaxTokens:    50000,
		CanReason:           true,
		SupportsAttachments: true,
	},
	Claude4Opus: {
		ID:                  Claude4Opus,
		Name:                "Claude 4 Opus",
		Provider:            ProviderAnthropic,
		APIModel:            "claude-opus-4-20250514",
		CostPer1MIn:         15.0,
		CostPer1MInCached:   18.75,
		CostPer1MOutCached:  1.50,
		CostPer1MOut:        75.0,
		ContextWindow:       200000,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
}
