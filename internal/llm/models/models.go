package models

import "maps"

type (
	ModelID           string
	InferenceProvider string
)

type Model struct {
	ID                  ModelID           `json:"id"`
	Name                string            `json:"name"`
	Provider            InferenceProvider `json:"provider"`
	APIModel            string            `json:"api_model"`
	CostPer1MIn         float64           `json:"cost_per_1m_in"`
	CostPer1MOut        float64           `json:"cost_per_1m_out"`
	CostPer1MInCached   float64           `json:"cost_per_1m_in_cached"`
	CostPer1MOutCached  float64           `json:"cost_per_1m_out_cached"`
	ContextWindow       int64             `json:"context_window"`
	DefaultMaxTokens    int64             `json:"default_max_tokens"`
	CanReason           bool              `json:"can_reason"`
	SupportsAttachments bool              `json:"supports_attachments"`
}

// Model IDs
const ( // GEMINI
	// Bedrock
	BedrockClaude37Sonnet ModelID = "bedrock.claude-3.7-sonnet"
)

const (
	ProviderBedrock InferenceProvider = "bedrock"
	// ForTests
	ProviderMock InferenceProvider = "__mock"
)

var SupportedModels = map[ModelID]Model{
	// Bedrock
	BedrockClaude37Sonnet: {
		ID:                 BedrockClaude37Sonnet,
		Name:               "Bedrock: Claude 3.7 Sonnet",
		Provider:           ProviderBedrock,
		APIModel:           "anthropic.claude-3-7-sonnet-20250219-v1:0",
		CostPer1MIn:        3.0,
		CostPer1MInCached:  3.75,
		CostPer1MOutCached: 0.30,
		CostPer1MOut:       15.0,
	},
}

var KnownProviders = []InferenceProvider{
	ProviderAnthropic,
	ProviderOpenAI,
	ProviderGemini,
	ProviderAzure,
	ProviderGROQ,
	ProviderLocal,
	ProviderOpenRouter,
	ProviderVertexAI,
	ProviderBedrock,
	ProviderXAI,
	ProviderMock,
}

func init() {
	maps.Copy(SupportedModels, AnthropicModels)
	maps.Copy(SupportedModels, OpenAIModels)
	maps.Copy(SupportedModels, GeminiModels)
	maps.Copy(SupportedModels, GroqModels)
	maps.Copy(SupportedModels, AzureModels)
	maps.Copy(SupportedModels, OpenRouterModels)
	maps.Copy(SupportedModels, XAIModels)
	maps.Copy(SupportedModels, VertexAIGeminiModels)
}
