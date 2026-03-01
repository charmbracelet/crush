package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestSetCompactionFlags(t *testing.T) {
	t.Parallel()

	t.Run("mutates csync values from auto to llm mode", func(t *testing.T) {
		t.Parallel()

		agent := &sessionAgent{
			disableAutoSummarize: csync.NewValue(false),
			disableContextStatus: csync.NewValue(true),
		}

		require.False(t, agent.disableAutoSummarize.Get())
		require.True(t, agent.disableContextStatus.Get())

		agent.SetCompactionFlags(true, false)

		require.True(t, agent.disableAutoSummarize.Get())
		require.False(t, agent.disableContextStatus.Get())
	})

	t.Run("mutates csync values from llm to auto mode", func(t *testing.T) {
		t.Parallel()

		agent := &sessionAgent{
			disableAutoSummarize: csync.NewValue(true),
			disableContextStatus: csync.NewValue(false),
		}

		require.True(t, agent.disableAutoSummarize.Get())
		require.False(t, agent.disableContextStatus.Get())

		agent.SetCompactionFlags(false, true)

		require.False(t, agent.disableAutoSummarize.Get())
		require.True(t, agent.disableContextStatus.Get())
	})
}

func TestCompactionModeSwitchRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("auto to llm via compactionFlags and SetCompactionFlags", func(t *testing.T) {
		t.Parallel()

		autoDisableAutoSummarize, autoDisableContextStatus := compactionFlags(config.CompactionAuto, false)

		agent := &sessionAgent{
			disableAutoSummarize: csync.NewValue(autoDisableAutoSummarize),
			disableContextStatus: csync.NewValue(autoDisableContextStatus),
		}

		require.False(t, agent.disableAutoSummarize.Get())
		require.True(t, agent.disableContextStatus.Get())

		llmDisableAutoSummarize, llmDisableContextStatus := compactionFlags(config.CompactionLLM, false)
		agent.SetCompactionFlags(llmDisableAutoSummarize, llmDisableContextStatus)

		require.True(t, agent.disableAutoSummarize.Get())
		require.False(t, agent.disableContextStatus.Get())
	})

	t.Run("llm to auto via compactionFlags and SetCompactionFlags", func(t *testing.T) {
		t.Parallel()

		llmDisableAutoSummarize, llmDisableContextStatus := compactionFlags(config.CompactionLLM, false)

		agent := &sessionAgent{
			disableAutoSummarize: csync.NewValue(llmDisableAutoSummarize),
			disableContextStatus: csync.NewValue(llmDisableContextStatus),
		}

		require.True(t, agent.disableAutoSummarize.Get())
		require.False(t, agent.disableContextStatus.Get())

		autoDisableAutoSummarize, autoDisableContextStatus := compactionFlags(config.CompactionAuto, false)
		agent.SetCompactionFlags(autoDisableAutoSummarize, autoDisableContextStatus)

		require.False(t, agent.disableAutoSummarize.Get())
		require.True(t, agent.disableContextStatus.Get())
	})

	t.Run("llm to auto with legacy override", func(t *testing.T) {
		t.Parallel()

		llmDisableAutoSummarize, llmDisableContextStatus := compactionFlags(config.CompactionLLM, false)

		agent := &sessionAgent{
			disableAutoSummarize: csync.NewValue(llmDisableAutoSummarize),
			disableContextStatus: csync.NewValue(llmDisableContextStatus),
		}

		autoDisableAutoSummarize, autoDisableContextStatus := compactionFlags(config.CompactionAuto, true)
		agent.SetCompactionFlags(autoDisableAutoSummarize, autoDisableContextStatus)

		require.True(t, agent.disableAutoSummarize.Get())
		require.True(t, agent.disableContextStatus.Get())
	})
}
