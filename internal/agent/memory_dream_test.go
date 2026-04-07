package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestShouldRunMemoryDream(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	lastAt := now.Add(-25 * time.Hour)
	recent := now.Add(-2 * time.Hour).Unix()
	old := now.Add(-48 * time.Hour).Unix()

	t.Run("time and sessions gate", func(t *testing.T) {
		require.True(t, shouldRunMemoryDream(now, lastAt, 5, false))
		require.False(t, shouldRunMemoryDream(now, now.Add(-2*time.Hour), 10, false))
		require.False(t, shouldRunMemoryDream(now, lastAt, 4, false))
	})

	t.Run("force bypasses gates", func(t *testing.T) {
		require.True(t, shouldRunMemoryDream(now, now, 0, true))
	})

	candidates := selectDreamCandidateSessions([]session.Session{
		{ID: "current", Title: "current", Kind: session.KindNormal, UpdatedAt: recent},
		{ID: "recent-a", Title: "Recent A", Kind: session.KindNormal, UpdatedAt: recent},
		{ID: "recent-b", Title: "Recent B", Kind: session.KindNormal, UpdatedAt: recent - 10},
		{ID: "old", Title: "Old", Kind: session.KindNormal, UpdatedAt: old},
		{ID: "child", Title: "Child", Kind: session.KindNormal, ParentSessionID: "p", UpdatedAt: recent},
		{ID: "handoff", Title: "Handoff", Kind: session.KindHandoff, UpdatedAt: recent},
	}, lastAt, "current")
	require.Len(t, candidates, 2)
	require.Equal(t, []string{"recent-a", "recent-b"}, []string{candidates[0].ID, candidates[1].ID})
}

func TestFormatMemoryFreshness(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	require.Empty(t, formatMemoryFreshness(now, time.Time{}, false))
	require.Contains(t, formatMemoryFreshness(now, time.Time{}, true), "never consolidated")
	require.Empty(t, formatMemoryFreshness(now, now.Add(-7*24*time.Hour), true))
	require.Contains(t, formatMemoryFreshness(now, now.Add(-47*24*time.Hour), true), "47 days")
}

func TestCoordinatorMemoryFreshness(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := &coordinator{longTermMemory: env.memory}

	status, err := coord.MemoryFreshness(context.Background())
	require.NoError(t, err)
	require.False(t, status.HasMemories)
	require.Empty(t, status.Warning)

	require.NoError(t, env.memory.Store(t.Context(), memory.StoreParams{Key: "project/context", Value: "Durable project context"}))
	oldAt := time.Now().Add(-47 * 24 * time.Hour)
	require.NoError(t, env.memory.WriteLastConsolidatedAt(oldAt))

	status, err = coord.MemoryFreshness(context.Background())
	require.NoError(t, err)
	require.True(t, status.HasMemories)
	require.Contains(t, status.Warning, "47 days")
}

func TestShouldSkipMemoryDreamSessionScan(t *testing.T) {
	t.Parallel()

	original := atomic.LoadInt64(&memoryDreamLastSessionScanUnix)
	t.Cleanup(func() {
		atomic.StoreInt64(&memoryDreamLastSessionScanUnix, original)
	})

	now := time.Now()
	markMemoryDreamSessionScan(now)
	require.True(t, shouldSkipMemoryDreamSessionScan(now.Add(time.Minute), false))
	require.False(t, shouldSkipMemoryDreamSessionScan(now.Add(11*time.Minute), false))
	require.False(t, shouldSkipMemoryDreamSessionScan(now.Add(time.Minute), true))
}
