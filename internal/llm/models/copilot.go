package models

const (
	ProviderCopilot ModelProvider = "copilot"

	// GitHub Copilot models
	CopilotGTP35Turbo      ModelID = "copilot.gpt-3.5-turbo"
	CopilotGPT4o           ModelID = "copilot.gpt-4o"
	CopilotGPT4oMini       ModelID = "copilot.gpt-4o-mini"
	CopilotGPT41           ModelID = "copilot.gpt-4.1"
	CopilotClaude35        ModelID = "copilot.claude-3.5-sonnet"
	CopilotClaude37        ModelID = "copilot.claude-3.7-sonnet"
	CopilotClaude4         ModelID = "copilot.claude-sonnet-4"
	CopilotO1              ModelID = "copilot.o1"
	CopilotO3Mini          ModelID = "copilot.o3-mini"
	CopilotO4Mini          ModelID = "copilot.o4-mini"
	CopilotGemini20        ModelID = "copilot.gemini-2.0-flash"
	CopilotGemini25        ModelID = "copilot.gemini-2.5-pro"
	CopilotGPT4            ModelID = "copilot.gpt-4"
	CopilotClaude37Thought ModelID = "copilot.claude-3.7-sonnet-thought"
)

var CopilotAnthropicModels = []ModelID{
	CopilotClaude35,
	CopilotClaude37,
	CopilotClaude37Thought,
	CopilotClaude4,
}

// GitHub Copilot models available through GitHub's API
var CopilotModels = map[ModelID]Model{
	CopilotGTP35Turbo: {
		ID:                  CopilotGTP35Turbo,
		Name:                "GitHub Copilot GPT-3.5-turbo",
		Provider:            ProviderCopilot,
		APIModel:            "gpt-3.5-turbo",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       16_384,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
	CopilotGPT4o: {
		ID:                  CopilotGPT4o,
		Name:                "GitHub Copilot GPT-4o",
		Provider:            ProviderCopilot,
		APIModel:            "gpt-4o",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    16384,
		SupportsAttachments: true,
	},
	CopilotGPT4oMini: {
		ID:                  CopilotGPT4oMini,
		Name:                "GitHub Copilot GPT-4o Mini",
		Provider:            ProviderCopilot,
		APIModel:            "gpt-4o-mini",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
	CopilotGPT41: {
		ID:                  CopilotGPT41,
		Name:                "GitHub Copilot GPT-4.1",
		Provider:            ProviderCopilot,
		APIModel:            "gpt-4.1",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    16384,
		CanReason:           true,
		SupportsAttachments: true,
	},
	CopilotClaude35: {
		ID:                  CopilotClaude35,
		Name:                "GitHub Copilot Claude 3.5 Sonnet",
		Provider:            ProviderCopilot,
		APIModel:            "claude-3.5-sonnet",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       90_000,
		DefaultMaxTokens:    8192,
		SupportsAttachments: true,
	},
	CopilotClaude37: {
		ID:                  CopilotClaude37,
		Name:                "GitHub Copilot Claude 3.7 Sonnet",
		Provider:            ProviderCopilot,
		APIModel:            "claude-3.7-sonnet",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       200_000,
		DefaultMaxTokens:    16384,
		SupportsAttachments: true,
	},
	CopilotClaude4: {
		ID:                  CopilotClaude4,
		Name:                "GitHub Copilot Claude Sonnet 4",
		Provider:            ProviderCopilot,
		APIModel:            "claude-sonnet-4",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    16000,
		SupportsAttachments: true,
	},
	CopilotO1: {
		ID:                  CopilotO1,
		Name:                "GitHub Copilot o1",
		Provider:            ProviderCopilot,
		APIModel:            "o1",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       200_000,
		DefaultMaxTokens:    100_000,
		CanReason:           true,
		SupportsAttachments: false,
	},
	CopilotO3Mini: {
		ID:                  CopilotO3Mini,
		Name:                "GitHub Copilot o3-mini",
		Provider:            ProviderCopilot,
		APIModel:            "o3-mini",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       200_000,
		DefaultMaxTokens:    100_000,
		CanReason:           true,
		SupportsAttachments: false,
	},
	CopilotO4Mini: {
		ID:                  CopilotO4Mini,
		Name:                "GitHub Copilot o4-mini",
		Provider:            ProviderCopilot,
		APIModel:            "o4-mini",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    16_384,
		CanReason:           true,
		SupportsAttachments: true,
	},
	CopilotGemini20: {
		ID:                  CopilotGemini20,
		Name:                "GitHub Copilot Gemini 2.0 Flash",
		Provider:            ProviderCopilot,
		APIModel:            "gemini-2.0-flash-001",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    8192,
		SupportsAttachments: true,
	},
	CopilotGemini25: {
		ID:                  CopilotGemini25,
		Name:                "GitHub Copilot Gemini 2.5 Pro",
		Provider:            ProviderCopilot,
		APIModel:            "gemini-2.5-pro",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    64000,
		SupportsAttachments: true,
	},
	CopilotGPT4: {
		ID:                  CopilotGPT4,
		Name:                "GitHub Copilot GPT-4",
		Provider:            ProviderCopilot,
		APIModel:            "gpt-4",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       32_768,
		DefaultMaxTokens:    4096,
		SupportsAttachments: true,
	},
	CopilotClaude37Thought: {
		ID:                  CopilotClaude37Thought,
		Name:                "GitHub Copilot Claude 3.7 Sonnet Thinking",
		Provider:            ProviderCopilot,
		APIModel:            "claude-3.7-sonnet-thought",
		CostPer1MIn:         0.0, // Included in GitHub Copilot subscription
		CostPer1MInCached:   0.0,
		CostPer1MOutCached:  0.0,
		CostPer1MOut:        0.0,
		ContextWindow:       200_000,
		DefaultMaxTokens:    16384,
		CanReason:           true,
		SupportsAttachments: true,
	},
}
