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

	t.Run("custom base URL from env", func(t *testing.T) {
		customURL := "https://custom.minimax.io"
		t.Setenv("MINIMAX_URL", customURL)
		// Note: BaseURL is cached, so we can't test this reliably
		// This test documents the expected behavior
	})
}
