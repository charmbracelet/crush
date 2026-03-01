package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCompactionFlags(t *testing.T) {
	t.Parallel()

	t.Run("auto mode disables context status and enables auto summarize", func(t *testing.T) {
		t.Parallel()

		disableAutoSummarize, disableContextStatus := compactionFlags(config.CompactionAuto, false)
		require.False(t, disableAutoSummarize)
		require.True(t, disableContextStatus)
	})

	t.Run("llm mode enables context status and disables auto summarize", func(t *testing.T) {
		t.Parallel()

		disableAutoSummarize, disableContextStatus := compactionFlags(config.CompactionLLM, false)
		require.True(t, disableAutoSummarize)
		require.False(t, disableContextStatus)
	})

	t.Run("empty method defaults to auto behavior", func(t *testing.T) {
		t.Parallel()

		disableAutoSummarize, disableContextStatus := compactionFlags("", false)
		require.False(t, disableAutoSummarize)
		require.True(t, disableContextStatus)
	})

	t.Run("auto mode respects legacy disable auto summarize override", func(t *testing.T) {
		t.Parallel()

		disableAutoSummarize, disableContextStatus := compactionFlags(config.CompactionAuto, true)
		require.True(t, disableAutoSummarize)
		require.True(t, disableContextStatus)
	})

	t.Run("llm mode ignores legacy disable auto summarize override", func(t *testing.T) {
		t.Parallel()

		disableAutoSummarize, disableContextStatus := compactionFlags(config.CompactionLLM, true)
		require.True(t, disableAutoSummarize)
		require.False(t, disableContextStatus)
	})
}
