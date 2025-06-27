package config

import (
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
