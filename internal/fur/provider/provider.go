// Package provider provides types and constants for AI providers.
package provider

// Type represents the type of AI provider.
type Type string

// All the supported AI provider types.
const (
	TypeOpenAI     Type = "openai"
	TypeAnthropic  Type = "anthropic"
	TypeGemini     Type = "gemini"
	TypeAzure      Type = "azure"
	TypeBedrock    Type = "bedrock"
	TypeVertexAI   Type = "vertexai"
	TypeXAI        Type = "xai"
	TypeOpenRouter Type = "openrouter"
)

// InferenceProvider represents the inference provider identifier.
type InferenceProvider string

// All the inference providers supported by the system.
const (
	InferenceProviderOpenAI     InferenceProvider = "openai"
	InferenceProviderAnthropic  InferenceProvider = "anthropic"
	InferenceProviderGemini     InferenceProvider = "gemini"
	InferenceProviderAzure      InferenceProvider = "azure"
	InferenceProviderBedrock    InferenceProvider = "bedrock"
	InferenceProviderVertexAI   InferenceProvider = "vertexai"
	InferenceProviderXAI        InferenceProvider = "xai"
	InferenceProviderOpenRouter InferenceProvider = "openrouter"
)

// Provider represents an AI provider configuration.
type Provider struct {
	Name                string            `json:"name"`
	ID                  InferenceProvider `json:"id"`
	APIKey              string            `json:"api_key,omitempty"`
	APIEndpoint         string            `json:"api_endpoint,omitempty"`
	Type                Type              `json:"type,omitempty"`
	DefaultLargeModelID string            `json:"default_large_model_id,omitempty"`
	DefaultSmallModelID string            `json:"default_small_model_id,omitempty"`
	Models              []Model           `json:"models,omitempty"`
}

// Model represents an AI model configuration.
type Model struct {
	ID                 string  `json:"id"`
	Name               string  `json:"model"`
	CostPer1MIn        float64 `json:"cost_per_1m_in"`
	CostPer1MOut       float64 `json:"cost_per_1m_out"`
	CostPer1MInCached  float64 `json:"cost_per_1m_in_cached"`
	CostPer1MOutCached float64 `json:"cost_per_1m_out_cached"`
	ContextWindow      int64   `json:"context_window"`
	DefaultMaxTokens   int64   `json:"default_max_tokens"`
	CanReason          bool    `json:"can_reason"`
	SupportsImages     bool    `json:"supports_attachments"`
}

// KnownProviders returns all the known inference providers.
func KnownProviders() []InferenceProvider {
	return []InferenceProvider{
		InferenceProviderOpenAI,
		InferenceProviderAnthropic,
		InferenceProviderGemini,
		InferenceProviderAzure,
		InferenceProviderBedrock,
		InferenceProviderVertexAI,
		InferenceProviderXAI,
		InferenceProviderOpenRouter,
	}
}
