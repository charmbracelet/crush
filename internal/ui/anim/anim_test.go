package anim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResetCache(t *testing.T) {
	// Reset to a known state.
	ResetCache()

	// Populate cache with two different settings.
	New(Settings{Size: 5, Label: "A"})
	New(Settings{Size: 5, Label: "B"})
	require.Equal(t, 2, animCacheMap.Len())

	ResetCache()
	require.Equal(t, 0, animCacheMap.Len())
}

func TestResetCacheIdempotent(t *testing.T) {
	// Resetting an empty cache should not panic.
	ResetCache()
	ResetCache()
	require.Equal(t, 0, animCacheMap.Len())
}

func TestResetCacheThenRecreate(t *testing.T) {
	// Reset to a known state.
	ResetCache()

	// Create an anim and cache it.
	a1 := New(Settings{Size: 5, Label: "Test"})
	require.Equal(t, 1, animCacheMap.Len())

	// Reset the cache.
	ResetCache()
	require.Equal(t, 0, animCacheMap.Len())

	// Create the same anim again; cache should repopulate.
	a2 := New(Settings{Size: 5, Label: "Test"})
	require.Equal(t, 1, animCacheMap.Len())

	// Both old and new anims should still render without panic.
	r1 := a1.Render()
	r2 := a2.Render()
	require.NotEmpty(t, r1)
	require.NotEmpty(t, r2)
}
