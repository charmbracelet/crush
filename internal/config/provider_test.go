package config

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockProviders(t *testing.T) {
	// Enable mock providers for testing
	originalUseMock := UseMockProviders
	UseMockProviders = true
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	// Reset providers to ensure we get fresh mock data
	ResetProviders()

	providers := Providers()
	require.NotEmpty(t, providers, "Mock providers should not be empty")

	// Verify we have the expected mock providers
	providerIDs := make(map[provider.InferenceProvider]bool)
	for _, p := range providers {
		providerIDs[p.ID] = true
	}

	assert.True(t, providerIDs[provider.InferenceProviderAnthropic], "Should have Anthropic provider")
	assert.True(t, providerIDs[provider.InferenceProviderOpenAI], "Should have OpenAI provider")
	assert.True(t, providerIDs[provider.InferenceProviderGemini], "Should have Gemini provider")

	// Verify Anthropic provider details
	var anthropicProvider provider.Provider
	for _, p := range providers {
		if p.ID == provider.InferenceProviderAnthropic {
			anthropicProvider = p
			break
		}
	}

	assert.Equal(t, "Anthropic", anthropicProvider.Name)
	assert.Equal(t, provider.TypeAnthropic, anthropicProvider.Type)
	assert.Equal(t, "claude-3-opus", anthropicProvider.DefaultLargeModelID)
	assert.Equal(t, "claude-3-haiku", anthropicProvider.DefaultSmallModelID)
	assert.Len(t, anthropicProvider.Models, 4, "Anthropic should have 4 models")

	// Verify model details
	var opusModel provider.Model
	for _, m := range anthropicProvider.Models {
		if m.ID == "claude-3-opus" {
			opusModel = m
			break
		}
	}

	assert.Equal(t, "Claude 3 Opus", opusModel.Name)
	assert.Equal(t, int64(200000), opusModel.ContextWindow)
	assert.Equal(t, int64(4096), opusModel.DefaultMaxTokens)
	assert.True(t, opusModel.SupportsImages)
}

func TestProvidersWithoutMock(t *testing.T) {
	// Ensure mock is disabled
	originalUseMock := UseMockProviders
	UseMockProviders = false
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	// Reset providers to ensure we get fresh data
	ResetProviders()

	// This will try to make an actual API call or use cached data
	providers := Providers()

	// We can't guarantee what we'll get here since it depends on network/cache
	// but we can at least verify the function doesn't panic
	t.Logf("Got %d providers without mock", len(providers))
}

func TestResetProviders(t *testing.T) {
	// Enable mock providers
	UseMockProviders = true
	defer func() {
		UseMockProviders = false
		ResetProviders()
	}()

	// Get providers once
	providers1 := Providers()
	require.NotEmpty(t, providers1)

	// Reset and get again
	ResetProviders()
	providers2 := Providers()
	require.NotEmpty(t, providers2)

	// Should get the same mock data
	assert.Equal(t, len(providers1), len(providers2))
}

func TestReasoningEffortSupport(t *testing.T) {
	originalUseMock := UseMockProviders
	UseMockProviders = true
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	ResetProviders()
	providers := Providers()
	
	var openaiProvider provider.Provider
	for _, p := range providers {
		if p.ID == provider.InferenceProviderOpenAI {
			openaiProvider = p
			break
		}
	}
	require.NotEmpty(t, openaiProvider.ID)

	var reasoningModel, nonReasoningModel provider.Model
	for _, model := range openaiProvider.Models {
		if model.CanReason && model.HasReasoningEffort {
			reasoningModel = model
		} else if !model.CanReason {
			nonReasoningModel = model
		}
	}

	require.NotEmpty(t, reasoningModel.ID)
	assert.Equal(t, "medium", reasoningModel.DefaultReasoningEffort)
	assert.True(t, reasoningModel.HasReasoningEffort)

	require.NotEmpty(t, nonReasoningModel.ID)
	assert.False(t, nonReasoningModel.HasReasoningEffort)
	assert.Empty(t, nonReasoningModel.DefaultReasoningEffort)
}

func TestReasoningEffortConfigTransfer(t *testing.T) {
	originalUseMock := UseMockProviders
	UseMockProviders = true
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	ResetProviders()
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	cfg, err := Init(t.TempDir(), false)
	require.NoError(t, err)

	openaiProviderConfig, exists := cfg.Providers[provider.InferenceProviderOpenAI]
	require.True(t, exists)

	var foundReasoning, foundNonReasoning bool
	for _, model := range openaiProviderConfig.Models {
		if model.CanReason && model.HasReasoningEffort && model.ReasoningEffort != "" {
			assert.Equal(t, "medium", model.ReasoningEffort)
			assert.True(t, model.HasReasoningEffort)
			foundReasoning = true
		} else if !model.CanReason {
			assert.Empty(t, model.ReasoningEffort)
			assert.False(t, model.HasReasoningEffort)
			foundNonReasoning = true
		}
	}

	assert.True(t, foundReasoning, "Should find at least one reasoning model")
	assert.True(t, foundNonReasoning, "Should find at least one non-reasoning model")
}

func TestNewProviders(t *testing.T) {
	originalUseMock := UseMockProviders
	UseMockProviders = true
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	ResetProviders()
	providers := Providers()
	require.NotEmpty(t, providers)

	var xaiProvider, openRouterProvider provider.Provider
	for _, p := range providers {
		switch p.ID {
		case provider.InferenceProviderXAI:
			xaiProvider = p
		case provider.InferenceProviderOpenRouter:
			openRouterProvider = p
		}
	}

	require.NotEmpty(t, xaiProvider.ID)
	assert.Equal(t, "xAI", xaiProvider.Name)
	assert.Equal(t, "grok-beta", xaiProvider.DefaultLargeModelID)

	require.NotEmpty(t, openRouterProvider.ID)
	assert.Equal(t, "OpenRouter", openRouterProvider.Name)
	assert.Equal(t, "anthropic/claude-3.5-sonnet", openRouterProvider.DefaultLargeModelID)
}

func TestO1ModelsInMockProvider(t *testing.T) {
	originalUseMock := UseMockProviders
	UseMockProviders = true
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	ResetProviders()
	providers := Providers()
	
	var openaiProvider provider.Provider
	for _, p := range providers {
		if p.ID == provider.InferenceProviderOpenAI {
			openaiProvider = p
			break
		}
	}
	require.NotEmpty(t, openaiProvider.ID)

	modelTests := []struct {
		id   string
		name string
	}{
		{"o1-preview", "o1-preview"},
		{"o1-mini", "o1-mini"},
	}

	for _, test := range modelTests {
		var model provider.Model
		var found bool
		for _, m := range openaiProvider.Models {
			if m.ID == test.id {
				model = m
				found = true
				break
			}
		}
		require.True(t, found, "Should find %s model", test.id)
		assert.Equal(t, test.name, model.Name)
		assert.True(t, model.CanReason)
		assert.True(t, model.HasReasoningEffort)
		assert.Equal(t, "medium", model.DefaultReasoningEffort)
	}
}

func TestPreferredModelReasoningEffort(t *testing.T) {
	// Test that PreferredModel struct can hold reasoning effort
	preferredModel := PreferredModel{
		ModelID:         "o1-preview",
		Provider:        provider.InferenceProviderOpenAI,
		ReasoningEffort: "high",
	}

	assert.Equal(t, "o1-preview", preferredModel.ModelID)
	assert.Equal(t, provider.InferenceProviderOpenAI, preferredModel.Provider)
	assert.Equal(t, "high", preferredModel.ReasoningEffort)

	// Test JSON marshaling/unmarshaling
	jsonData, err := json.Marshal(preferredModel)
	require.NoError(t, err)

	var unmarshaled PreferredModel
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, preferredModel.ModelID, unmarshaled.ModelID)
	assert.Equal(t, preferredModel.Provider, unmarshaled.Provider)
	assert.Equal(t, preferredModel.ReasoningEffort, unmarshaled.ReasoningEffort)
}
