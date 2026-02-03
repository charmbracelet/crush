package minimax

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnabled(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		// Clear any environment variables that might enable it
		t.Setenv("MINIMAX", "")
		t.Setenv("MINIMAX_ENABLE", "")
		t.Setenv("MINIMAX_ENABLED", "")

		// Enabled is a sync.OnceValue, so we need to test in isolation
		require.False(t, Enabled())
	})
}

func TestEmbedded(t *testing.T) {
	provider := Embedded()
	require.Equal(t, "minimax", string(provider.ID))
	require.Equal(t, "MiniMax Coding Plan", provider.Name)
	require.NotEmpty(t, provider.Models)
	require.Len(t, provider.Models, 2)

	// Check MiniMax-M2 model
	m2Found := false
	m21Found := false
	for _, model := range provider.Models {
		if model.ID == "MiniMax-M2" {
			m2Found = true
			require.Equal(t, "MiniMax M2", model.Name)
			require.Equal(t, int64(197000), model.ContextWindow)
		}
		if model.ID == "MiniMax-M2.1" {
			m21Found = true
			require.Equal(t, "MiniMax M2.1", model.Name)
			require.Equal(t, int64(205000), model.ContextWindow)
		}
	}
	require.True(t, m2Found, "MiniMax-M2 model should be present")
	require.True(t, m21Found, "MiniMax-M2.1 model should be present")
}

func TestBaseURL(t *testing.T) {
	t.Run("default base URL", func(t *testing.T) {
		t.Setenv("MINIMAX_URL", "")
		require.Equal(t, "https://api.minimax.io/anthropic", BaseURL())
	})

	// Note: BaseURL uses sync.OnceValue which caches the result on first call.
	// To test custom URLs, integration tests should be used where the process
	// is started with the environment variable already set.
	t.Run("custom base URL behavior", func(t *testing.T) {
		// This test documents the expected behavior when MINIMAX_URL is set
		// before the application starts. The actual URL used will be determined
		// at the time BaseURL is first called, so environment changes after
		// that point will not affect the cached value.
		t.Skip("BaseURL uses sync.OnceValue and is cached - tested via integration")
	})
}
