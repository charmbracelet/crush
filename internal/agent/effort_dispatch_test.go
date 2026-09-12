package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the effort-application contract that buildAgent relies on:
// it calls subagents.ApplyEffortToModel on the resolved primary model
// (primary.ModelCfg, primary.CatwalkCfg). The cases mirror the dispatch path
// taken when a named subagent has Effort set.

func TestApplyEffortToModel_HighEffort_OpenAI(t *testing.T) {
	t.Parallel()

	cfg := config.SelectedModel{Model: "o4-mini", Provider: "openai"}
	cat := catwalk.Model{
		ID:              "o4-mini",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high"},
	}

	got := subagents.ApplyEffortToModel("high", cfg, cat)

	require.Equal(t, "high", got.ReasoningEffort,
		"dispatch path must propagate high effort to ReasoningEffort")
}

func TestApplyEffortToModel_HighEffort_Anthropic(t *testing.T) {
	t.Parallel()

	cfg := config.SelectedModel{Model: "claude-opus-4-7", Provider: "anthropic"}
	cat := catwalk.Model{
		ID:              "claude-opus-4-7",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high", "xhigh", "max"},
	}

	got := subagents.ApplyEffortToModel("high", cfg, cat)

	require.Equal(t, "high", got.ReasoningEffort,
		"must set ReasoningEffort for an Anthropic model with high effort")
	assert.False(t, got.Think, "Think must never be set by ApplyEffortToModel")
}

func TestApplyEffortToModel_EmptyEffort_NoOp(t *testing.T) {
	t.Parallel()

	cfg := config.SelectedModel{Model: "o4-mini", Provider: "openai"}
	cat := catwalk.Model{ID: "o4-mini", CanReason: true, ReasoningLevels: []string{"low", "high"}}

	got := subagents.ApplyEffortToModel("", cfg, cat)

	assert.Empty(t, got.ReasoningEffort, "empty effort must not set ReasoningEffort")
	assert.False(t, got.Think, "empty effort must not set Think")
}

func TestApplyEffortToModel_PreservesOtherFields(t *testing.T) {
	t.Parallel()

	cfg := config.SelectedModel{Model: "o4-mini", Provider: "openai", MaxTokens: 4096}
	cat := catwalk.Model{ID: "o4-mini", CanReason: true, ReasoningLevels: []string{"low", "medium", "high"}}

	got := subagents.ApplyEffortToModel("high", cfg, cat)

	assert.Equal(t, "o4-mini", got.Model)
	assert.Equal(t, "openai", got.Provider)
	assert.Equal(t, int64(4096), got.MaxTokens)
	assert.Equal(t, "high", got.ReasoningEffort)
}

func TestApplyEffortToModel_NonReasoningModel(t *testing.T) {
	t.Parallel()

	cfg := config.SelectedModel{Model: "gpt-4o", Provider: "openai"}
	cat := catwalk.Model{ID: "gpt-4o", CanReason: false}

	got := subagents.ApplyEffortToModel("high", cfg, cat)

	assert.Empty(t, got.ReasoningEffort, "non-reasoning model must not have ReasoningEffort set")
	assert.False(t, got.Think, "non-reasoning model must not have Think set")
}
