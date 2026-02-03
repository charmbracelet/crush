package config

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxSync(t *testing.T) {
	t.Run("returns embedded provider when autoupdate disabled", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_DATA_HOME", tmpDir)
		t.Setenv("MINIMAX", "1")

		syncer := &minimaxSync{}
		syncer.Init(cachePathFor("minimax"), false)

		provider, err := syncer.Get(context.Background())
		require.NoError(t, err)
		require.Equal(t, "minimax", string(provider.ID))
		require.Equal(t, "MiniMax Coding Plan", provider.Name)
		require.Len(t, provider.Models, 2)
	})

	t.Run("returns embedded provider when autoupdate enabled", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_DATA_HOME", tmpDir)
		t.Setenv("MINIMAX", "1")

		syncer := &minimaxSync{}
		syncer.Init(cachePathFor("minimax"), true)

		provider, err := syncer.Get(context.Background())
		require.NoError(t, err)
		require.Equal(t, "minimax", string(provider.ID))
		require.Equal(t, "MiniMax Coding Plan", provider.Name)
		require.Len(t, provider.Models, 2)
	})
}

func TestProviders_WithMiniMax(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("MINIMAX", "1")

	// Use a test-specific instance to avoid global state interference.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}
	testMinimaxSyncer := &minimaxSync{}

	originalCatwalkSyncer := catwalkSyncer
	originalHyperSyncer := hyperSyncer
	originalMinimaxSyncer := minimaxSyncer
	defer func() {
		catwalkSyncer = originalCatwalkSyncer
		hyperSyncer = originalHyperSyncer
		minimaxSyncer = originalMinimaxSyncer
	}()

	catwalkSyncer = testCatwalkSyncer
	hyperSyncer = testHyperSyncer
	minimaxSyncer = testMinimaxSyncer

	resetProviderState()
	defer resetProviderState()

	cfg := &Config{
		Options: &Options{
			DisableProviderAutoUpdate: true,
		},
	}

	providers, err := Providers(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, providers)

	// Check if MiniMax provider is present
	minimaxFound := false
	for _, provider := range providers {
		if string(provider.ID) == "minimax" {
			minimaxFound = true
			require.Equal(t, "MiniMax Coding Plan", provider.Name)
			require.Len(t, provider.Models, 2)
			break
		}
	}
	require.True(t, minimaxFound, "MiniMax provider should be present when enabled")
}

func TestUpdateMiniMax(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("MINIMAX", "1")

	t.Run("updates successfully with embedded", func(t *testing.T) {
		err := UpdateMiniMax("embedded")
		require.NoError(t, err)

		// Verify the cache was written
		cache := newCache[catwalk.Provider](cachePathFor("minimax"))
		provider, _, err := cache.Get()
		require.NoError(t, err)
		require.Equal(t, "minimax", string(provider.ID))
	})

	// Note: Testing the "not enabled" case is skipped because Enabled() uses
	// sync.OnceValue which caches the result on first call. This behavior is
	// better tested via integration tests where the process starts with the
	// appropriate environment variables.
}
