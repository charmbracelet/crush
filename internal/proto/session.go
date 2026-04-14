package proto

// SelectedModelType represents the type of model selection (large or small).
type SelectedModelType string

const (
	SelectedModelTypeLarge SelectedModelType = "large"
	SelectedModelTypeSmall SelectedModelType = "small"
)

// SelectedModel represents a model selection with provider and configuration.
type SelectedModel struct {
	Model            string         `json:"model"`
	Provider         string         `json:"provider"`
	ReasoningEffort  string         `json:"reasoning_effort,omitempty"`
	Think            bool           `json:"think,omitempty"`
	MaxTokens        int64          `json:"max_tokens,omitempty"`
	Temperature      *float64       `json:"temperature,omitempty"`
	TopP             *float64       `json:"top_p,omitempty"`
	TopK             *int64         `json:"top_k,omitempty"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty"`
	ProviderOptions  map[string]any `json:"provider_options,omitempty"`
}

// Session represents a session in the proto layer.
type Session struct {
	ID               string                              `json:"id"`
	ParentSessionID  string                              `json:"parent_session_id"`
	Title            string                              `json:"title"`
	MessageCount     int64                               `json:"message_count"`
	PromptTokens     int64                               `json:"prompt_tokens"`
	CompletionTokens int64                               `json:"completion_tokens"`
	SummaryMessageID string                              `json:"summary_message_id"`
	Cost             float64                             `json:"cost"`
	CreatedAt        int64                               `json:"created_at"`
	UpdatedAt        int64                               `json:"updated_at"`
	Models           map[SelectedModelType]SelectedModel `json:"models,omitempty"`
}
