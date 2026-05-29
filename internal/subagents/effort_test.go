package subagents

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestParseContent_EffortField verifies that the effort field round-trips
// through YAML frontmatter parsing for all defined values plus absent/empty.
func TestParseContent_EffortField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantEffort string
	}{
		{
			name: "effort_none",
			content: `---
name: my-agent
description: A test agent.
effort: none
---
`,
			wantEffort: "none",
		},
		{
			name: "effort_minimal",
			content: `---
name: my-agent
description: A test agent.
effort: minimal
---
`,
			wantEffort: "minimal",
		},
		{
			name: "effort_low",
			content: `---
name: my-agent
description: A test agent.
effort: low
---
`,
			wantEffort: "low",
		},
		{
			name: "effort_medium",
			content: `---
name: my-agent
description: A test agent.
effort: medium
---
`,
			wantEffort: "medium",
		},
		{
			name: "effort_high",
			content: `---
name: my-agent
description: A test agent.
effort: high
---
`,
			wantEffort: "high",
		},
		{
			name: "effort_xhigh",
			content: `---
name: my-agent
description: A test agent.
effort: xhigh
---
`,
			wantEffort: "xhigh",
		},
		{
			name: "effort_max",
			content: `---
name: my-agent
description: A test agent.
effort: max
---
`,
			wantEffort: "max",
		},
		{
			name: "effort_absent_is_empty",
			content: `---
name: my-agent
description: A test agent.
---
`,
			wantEffort: "",
		},
		{
			name: "effort_explicit_empty_string",
			content: `---
name: my-agent
description: A test agent.
effort: ""
---
`,
			wantEffort: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content))
			require.NoError(t, err)
			require.Equal(t, tt.wantEffort, agent.Effort)
		})
	}
}

// TestValidate_EffortField verifies that Validate accepts all seven defined
// effort constants and empty, and rejects everything else.
func TestValidate_EffortField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		effort  string
		wantErr bool
		errMsg  string
	}{
		{name: "empty_accepted", effort: "", wantErr: false},
		{name: "none_accepted", effort: "none", wantErr: false},
		{name: "minimal_accepted", effort: "minimal", wantErr: false},
		{name: "low_accepted", effort: "low", wantErr: false},
		{name: "medium_accepted", effort: "medium", wantErr: false},
		{name: "high_accepted", effort: "high", wantErr: false},
		{name: "xhigh_accepted", effort: "xhigh", wantErr: false},
		{name: "max_accepted", effort: "max", wantErr: false},
		{name: "ultra_rejected", effort: "ultra", wantErr: true, errMsg: "effort"},
		{name: "turbo_rejected", effort: "turbo", wantErr: true, errMsg: "effort"},
		{name: "HIGH_rejected_case_sensitive", effort: "HIGH", wantErr: true, errMsg: "effort"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := Subagent{
				Name:        "test-agent",
				Description: "Does something.",
				Effort:      tt.effort,
			}
			err := s.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestApplyEffortToModel_OpenAI verifies that effort values pass through
// directly as ReasoningEffort for an OpenAI-family model. Think is never set.
func TestApplyEffortToModel_OpenAI(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:              "o4-mini",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high"},
	}

	tests := []struct {
		effort        string
		wantReasoning string
	}{
		{"none", "none"},
		{"minimal", "minimal"},
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"xhigh", "xhigh"},
		{"max", "max"},
	}

	for _, tt := range tests {
		t.Run("effort_"+tt.effort, func(t *testing.T) {
			t.Parallel()

			base := config.SelectedModel{
				Model:    "o4-mini",
				Provider: "openai",
			}
			result := ApplyEffortToModel(tt.effort, base, m)
			require.Equal(t, tt.wantReasoning, result.ReasoningEffort)
			require.False(t, result.Think, "Think must never be set by ApplyEffortToModel")
		})
	}
}

// TestApplyEffortToModel_Anthropic verifies that effort values pass through
// directly as ReasoningEffort for Anthropic models — Think is never set.
func TestApplyEffortToModel_Anthropic(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:              "claude-opus-4-7",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high", "xhigh", "max"},
	}

	tests := []struct {
		effort        string
		wantReasoning string
	}{
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"xhigh", "xhigh"},
		{"max", "max"},
	}

	for _, tt := range tests {
		t.Run("effort_"+tt.effort, func(t *testing.T) {
			t.Parallel()

			base := config.SelectedModel{
				Model:    "claude-opus-4-7",
				Provider: "anthropic",
			}
			result := ApplyEffortToModel(tt.effort, base, m)
			require.Equal(t, tt.wantReasoning, result.ReasoningEffort)
			require.False(t, result.Think, "Think must never be set by ApplyEffortToModel")
		})
	}
}

// TestApplyEffortToModel_EmptyEffort_NoOp verifies that an empty effort string
// returns the model unchanged.
func TestApplyEffortToModel_EmptyEffort_NoOp(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:        "o4-mini",
		CanReason: true,
	}
	base := config.SelectedModel{
		Model:    "o4-mini",
		Provider: "openai",
	}

	result := ApplyEffortToModel("", base, m)
	require.Empty(t, result.ReasoningEffort, "empty effort must not set ReasoningEffort")
	require.False(t, result.Think, "empty effort must not set Think")
}

// TestApplyEffortToModel_NonReasoningModel verifies that effort has no effect
// on models that do not support reasoning (CanReason == false).
func TestApplyEffortToModel_NonReasoningModel(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:        "gpt-4o",
		CanReason: false,
	}
	base := config.SelectedModel{
		Model:    "gpt-4o",
		Provider: "openai",
	}

	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("effort_"+effort, func(t *testing.T) {
			t.Parallel()

			result := ApplyEffortToModel(effort, base, m)
			require.Empty(t, result.ReasoningEffort, "non-reasoning model must not have ReasoningEffort set")
			require.False(t, result.Think, "non-reasoning model must not have Think set")
		})
	}
}

// TestApplyEffortToModel_PreservesOtherFields verifies that ApplyEffortToModel
// does not mutate fields unrelated to effort on the SelectedModel.
func TestApplyEffortToModel_PreservesOtherFields(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:              "o4-mini",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high"},
	}
	base := config.SelectedModel{
		Model:     "o4-mini",
		Provider:  "openai",
		MaxTokens: 8192,
	}

	result := ApplyEffortToModel("high", base, m)
	require.Equal(t, base.Model, result.Model)
	require.Equal(t, base.Provider, result.Provider)
	require.Equal(t, base.MaxTokens, result.MaxTokens)
	require.Equal(t, "high", result.ReasoningEffort)
}

// TestApplyEffortToModel_XHighAndMaxPassThrough verifies that xhigh and max
// are set verbatim as ReasoningEffort without any clamping or mapping.
func TestApplyEffortToModel_XHighAndMaxPassThrough(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:              "o4-mini",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high"},
	}
	base := config.SelectedModel{
		Model:    "o4-mini",
		Provider: "openai",
	}

	t.Run("xhigh", func(t *testing.T) {
		t.Parallel()

		result := ApplyEffortToModel("xhigh", base, m)
		require.Equal(t, "xhigh", result.ReasoningEffort)
	})

	t.Run("max", func(t *testing.T) {
		t.Parallel()

		result := ApplyEffortToModel("max", base, m)
		require.Equal(t, "max", result.ReasoningEffort)
	})
}

// TestApplyEffortToModel_EmptyReasoningLevels verifies that when a model has
// CanReason=true but no ReasoningLevels list, effort still passes through as
// ReasoningEffort. The coordinator's shouldSetEffort check handles filtering at
// dispatch time; ApplyEffortToModel itself does not clamp.
func TestApplyEffortToModel_EmptyReasoningLevels(t *testing.T) {
	t.Parallel()

	m := catwalk.Model{
		ID:              "some-reasoning-model",
		CanReason:       true,
		ReasoningLevels: nil,
	}
	base := config.SelectedModel{
		Model:    "some-reasoning-model",
		Provider: "openai",
	}

	result := ApplyEffortToModel("high", base, m)
	require.Equal(t, "high", result.ReasoningEffort)
	require.False(t, result.Think)
}

// TestDispatchAppliesEffort_EndToEnd verifies the end-to-end path: when a
// Subagent has an Effort field set, combining ToConfigAgent and
// ApplyEffortToModel produces a model with ReasoningEffort set and Think unset.
func TestDispatchAppliesEffort_EndToEnd(t *testing.T) {
	t.Parallel()

	sa := &Subagent{
		Name:        "sharp-agent",
		Description: "An effort-aware subagent.",
		Effort:      "high",
	}

	base := config.Agent{
		AllowedTools: []string{"bash", "grep"},
		Model:        config.SelectedModelTypeLarge,
	}
	agentCfg := sa.ToConfigAgent(base)
	require.Equal(t, "sharp-agent", agentCfg.ID)

	resolvedModel := config.SelectedModel{
		Model:    "o4-mini",
		Provider: "openai",
	}
	catwalkModel := catwalk.Model{
		ID:              "o4-mini",
		CanReason:       true,
		ReasoningLevels: []string{"low", "medium", "high"},
	}

	applied := ApplyEffortToModel(sa.Effort, resolvedModel, catwalkModel)
	require.Equal(t, "high", applied.ReasoningEffort)
	require.False(t, applied.Think)
}

// TestDispatchAppliesEffort_Anthropic_EndToEnd verifies that the Anthropic
// path also sets ReasoningEffort (not Think) for effort values in the model's
// supported range.
func TestDispatchAppliesEffort_Anthropic_EndToEnd(t *testing.T) {
	t.Parallel()

	for _, effort := range []string{"medium", "high", "xhigh", "max"} {
		t.Run("effort_"+effort, func(t *testing.T) {
			t.Parallel()

			resolvedModel := config.SelectedModel{
				Model:    "claude-opus-4-7",
				Provider: "anthropic",
			}
			catwalkModel := catwalk.Model{
				ID:              "claude-opus-4-7",
				CanReason:       true,
				ReasoningLevels: []string{"low", "medium", "high", "xhigh", "max"},
			}

			applied := ApplyEffortToModel(effort, resolvedModel, catwalkModel)
			require.Equal(t, effort, applied.ReasoningEffort, "effort=%q must set ReasoningEffort for Anthropic models", effort)
			require.False(t, applied.Think, "Think must never be set by ApplyEffortToModel")
		})
	}
}

// TestEffortIgnored verifies the capability check used to warn on misconfig:
// a non-empty effort on a non-reasoning model is "ignored".
func TestEffortIgnored(t *testing.T) {
	t.Parallel()

	reasoning := catwalk.Model{ID: "r", CanReason: true}
	plain := catwalk.Model{ID: "p", CanReason: false}

	require.True(t, EffortIgnored("high", plain), "effort on non-reasoning model is ignored")
	require.False(t, EffortIgnored("high", reasoning), "effort on reasoning model is honored")
	require.False(t, EffortIgnored("", plain), "empty effort is never a misconfig")
	require.False(t, EffortIgnored("", reasoning))
}
