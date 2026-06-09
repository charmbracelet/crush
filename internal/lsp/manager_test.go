package lsp

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestUnavailableBackoff(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	now := base

	manager := &Manager{
		unavailable: csync.NewMap[string, time.Time](),
		now:         func() time.Time { return now },
		unavailableRetry: defaultUnavailableRetryDelay,
	}

	require.False(t, manager.recentlyUnavailable("gopls"))

	manager.markUnavailable("gopls")
	require.True(t, manager.recentlyUnavailable("gopls"))

	// With the default infinite backoff, it should still be unavailable
	// even after a very long time.
	now = now.Add(time.Hour * 24 * 365 * 100)
	require.True(t, manager.recentlyUnavailable("gopls"))

	// Clearing should make it available immediately.
	manager.clearUnavailable("gopls")
	require.False(t, manager.recentlyUnavailable("gopls"))
}

func TestUnavailableBackoffCustomDelay(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	now := base

	manager := &Manager{
		unavailable:      csync.NewMap[string, time.Time](),
		now:              func() time.Time { return now },
		unavailableRetry: 5 * time.Second,
	}

	manager.markUnavailable("gopls")
	require.True(t, manager.recentlyUnavailable("gopls"))

	now = now.Add(4 * time.Second)
	require.True(t, manager.recentlyUnavailable("gopls"))

	now = now.Add(2 * time.Second)
	require.False(t, manager.recentlyUnavailable("gopls"))
}

func TestUnavailableBackoffZeroMeansNoBackoff(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	now := base

	manager := &Manager{
		unavailable:      csync.NewMap[string, time.Time](),
		now:              func() time.Time { return now },
		unavailableRetry: 0,
	}

	manager.markUnavailable("gopls")
	require.False(t, manager.recentlyUnavailable("gopls"))
}
