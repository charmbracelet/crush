package agent

import (
	"context"
	"testing"
	"time"

	"github.com/taigrr/crush/internal/agent/prompt"
	"github.com/taigrr/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestSystemPrompts_NoPanic verifies that all system prompt templates
// render without errors. This catches mismatches between template fields
// and their corresponding data structs.
func TestSystemPrompts_NoPanic(t *testing.T) {
	t.Parallel()

	fixedTime := func() time.Time {
		t, _ := time.Parse("1/2/2006", "1/1/2025")
		return t
	}

	workingDir := t.TempDir()
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	t.Run("coder", func(t *testing.T) {
		t.Parallel()
		p, err := coderPrompt(
			prompt.WithTimeFunc(fixedTime),
			prompt.WithPlatform("linux"),
			prompt.WithWorkingDir(workingDir),
		)
		require.NoError(t, err)

		result, err := p.Build(context.Background(), "test-provider", "test-model", cfg)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("task", func(t *testing.T) {
		t.Parallel()
		p, err := taskPrompt(
			prompt.WithTimeFunc(fixedTime),
			prompt.WithPlatform("linux"),
			prompt.WithWorkingDir(workingDir),
		)
		require.NoError(t, err)

		result, err := p.Build(context.Background(), "test-provider", "test-model", cfg)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("initialize", func(t *testing.T) {
		t.Parallel()
		result, err := InitializePrompt(cfg)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})

	t.Run("agentic_fetch", func(t *testing.T) {
		t.Parallel()
		p, err := prompt.NewPrompt("agentic_fetch", string(agenticFetchPromptTmpl),
			prompt.WithTimeFunc(fixedTime),
			prompt.WithPlatform("linux"),
			prompt.WithWorkingDir(workingDir),
		)
		require.NoError(t, err)

		result, err := p.Build(context.Background(), "test-provider", "test-model", cfg)
		require.NoError(t, err)
		require.NotEmpty(t, result)
	})
}
