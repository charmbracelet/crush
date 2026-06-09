package lsp

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestParseUnavailableRetryDelay(t *testing.T) {
	t.Parallel()

	negOne := -1
	zero := 0
	five := 5

	require.Equal(t, defaultUnavailableRetryDelay, parseUnavailableRetryDelay(nil))
	require.Equal(t, defaultUnavailableRetryDelay, parseUnavailableRetryDelay(&negOne))
	require.Equal(t, time.Duration(0), parseUnavailableRetryDelay(&zero))
	require.Equal(t, 5*time.Second, parseUnavailableRetryDelay(&five))

	if strconv.IntSize == 64 {
		huge := int(math.MaxInt64)
		require.Equal(t, defaultUnavailableRetryDelay, parseUnavailableRetryDelay(&huge))
	}
}

func TestNewManager_DefaultRetryDelay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := config.Load(dir, "", false)
	require.NoError(t, err)

	m := NewManager(store)
	require.Equal(t, defaultUnavailableRetryDelay, m.unavailableRetry)
}

func TestNewManager_CustomRetryDelay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "crush.json"),
		[]byte(`{"options":{"lsp_unavailable_retry_delay":5}}`),
		0o644,
	))

	store, err := config.Load(dir, "", false)
	require.NoError(t, err)

	m := NewManager(store)
	require.Equal(t, 5*time.Second, m.unavailableRetry)
}

func TestUnavailableBackoff(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	now := base

	manager := &Manager{
		unavailable:      csync.NewMap[string, time.Time](),
		now:              func() time.Time { return now },
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
	_, exists := manager.unavailable.Get("gopls")
	require.False(t, exists)
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
