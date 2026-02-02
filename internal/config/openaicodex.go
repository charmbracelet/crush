package config

import "charm.land/catwalk/pkg/catwalk"

const (
	OpenAICodexProviderID   = "openai-codex"
	openAICodexBaseURL      = "https://chatgpt.com/backend-api/codex"
	openAICodexAPIKeyEnv    = "$OPENAI_CODEX_ACCESS_TOKEN"
	openAICodexInstructions = "You are a helpful coding assistant."
)

// OpenAICodexProvider returns the catwalk provider for the OpenAI Codex service.
func OpenAICodexProvider() catwalk.Provider {
	models := []catwalk.Model{
		openAICodexModel("gpt-5.2", "GPT-5.2", []string{"none", "low", "medium", "high", "xhigh"}, "medium"),
		openAICodexModel("gpt-5.2-codex", "GPT-5.2 Codex", []string{"low", "medium", "high", "xhigh"}, "medium"),
		openAICodexModel("gpt-5.1-codex-max", "GPT-5.1 Codex Max", []string{"low", "medium", "high", "xhigh"}, "medium"),
		openAICodexModel("gpt-5.1-codex", "GPT-5.1 Codex", []string{"low", "medium", "high"}, "medium"),
		openAICodexModel("gpt-5.1-codex-mini", "GPT-5.1 Codex Mini", []string{"medium", "high"}, "medium"),
		openAICodexModel("gpt-5.1", "GPT-5.1", []string{"none", "low", "medium", "high"}, "medium"),
	}

	return catwalk.Provider{
		ID:                  catwalk.InferenceProvider(OpenAICodexProviderID),
		Name:                "OpenAI Codex (ChatGPT OAuth)",
		APIEndpoint:         openAICodexBaseURL,
		APIKey:              openAICodexAPIKeyEnv,
		Type:                catwalk.TypeOpenAICompat,
		Models:              models,
		DefaultLargeModelID: "gpt-5.2-codex",
		DefaultSmallModelID: "gpt-5.1-codex-mini",
	}
}

func IsOpenAICodexProvider(providerID string) bool {
	return providerID == OpenAICodexProviderID
}

func openAICodexModel(id, name string, reasoningLevels []string, defaultEffort string) catwalk.Model {
	return catwalk.Model{
		ID:                     id,
		Name:                   name,
		ContextWindow:          272000,
		DefaultMaxTokens:       128000,
		CanReason:              len(reasoningLevels) > 0,
		ReasoningLevels:        reasoningLevels,
		DefaultReasoningEffort: defaultEffort,
		SupportsImages:         true,
	}
}
