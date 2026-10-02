package hooks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCompactionPayload(t *testing.T) {
	t.Parallel()

	data := BuildCompactionPayload(CompactionDetail{
		Trigger:      "auto",
		Instructions: "keep the failing test names",
		Messages:     12,
		Tokens:       40_000,
	})

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &got))
	assert.Equal(t, "auto", got["trigger"])
	assert.Equal(t, "keep the failing test names", got["instructions"])
	assert.Equal(t, float64(12), got["messages"])
	assert.Equal(t, float64(40_000), got["tokens"])
}

// TestRunCompactionWithoutHooks covers the path every session takes: no
// configured commands means no shell, and no opinion.
func TestRunCompactionWithoutHooks(t *testing.T) {
	t.Parallel()

	runner := NewRunner(nil, t.TempDir(), t.TempDir())
	result, err := runner.RunCompaction(t.Context(), EventPreCompact, "s-1", CompactionDetail{Trigger: "manual"})
	require.NoError(t, err)
	assert.Equal(t, DecisionNone, result.Decision)
}

func TestCompactionEventNames(t *testing.T) {
	t.Parallel()

	// The wire names are what config normalization and the docs promise.
	assert.Equal(t, "PreCompact", EventPreCompact)
	assert.Equal(t, "PostCompact", EventPostCompact)
}
