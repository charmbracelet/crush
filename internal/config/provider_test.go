package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviders_MockEnabled(t *testing.T) {
	originalUseMock := UseMockProviders
	UseMockProviders = true
	defer func() {
		UseMockProviders = originalUseMock
		ResetProviders()
	}()

	ResetProviders()
	providers := Providers()
	require.NotEmpty(t, providers)

	providerIDs := make(map[provider.InferenceProvider]bool)
	for _, p := range providers {
		providerIDs[p.ID] = true
	}

	assert.True(t, providerIDs[provider.InferenceProviderAnthropic])
	assert.True(t, providerIDs[provider.InferenceProviderOpenAI])
	assert.True(t, providerIDs[provider.InferenceProviderGemini])
}

func TestProviders_ResetFunctionality(t *testing.T) {
	UseMockProviders = true
	defer func() {
		UseMockProviders = false
		ResetProviders()
	}()

	providers1 := Providers()
	require.NotEmpty(t, providers1)

	ResetProviders()
	providers2 := Providers()
	require.NotEmpty(t, providers2)

	assert.Equal(t, len(providers1), len(providers2))
}

func TestProviders_ModelCapabilities(t *testing.T) {
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

	var foundReasoning, foundNonReasoning bool
	for _, model := range openaiProvider.Models {
		if model.CanReason && model.HasReasoningEffort {
			foundReasoning = true
		} else if !model.CanReason {
			foundNonReasoning = true
		}
	}

	assert.True(t, foundReasoning)
	assert.True(t, foundNonReasoning)
}