package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestSplitModelEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      string
		wantModel  string
		wantEffort string
	}{
		{name: "empty", model: "", wantModel: "", wantEffort: ""},
		{name: "no suffix", model: "gpt-4o", wantModel: "gpt-4o", wantEffort: ""},
		{name: "provider and model with suffix", model: "anthropic/claude-opus-5:xhigh", wantModel: "anthropic/claude-opus-5", wantEffort: "xhigh"},
		{name: "model with suffix", model: "claude-opus-5:high", wantModel: "claude-opus-5", wantEffort: "high"},
		{name: "off suffix", model: "openai/gpt-4o:off", wantModel: "openai/gpt-4o", wantEffort: "off"},
		{name: "unknown suffix preserved", model: "openrouter/model:free", wantModel: "openrouter/model:free", wantEffort: ""},
		{name: "exacto suffix preserved", model: "openrouter/deepseek/deepseek-v3.1-terminus:exacto", wantModel: "openrouter/deepseek/deepseek-v3.1-terminus:exacto", wantEffort: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean, effort := SplitModelEffort(tt.model)
			require.Equal(t, tt.wantModel, clean)
			require.Equal(t, tt.wantEffort, effort)
		})
	}
}

func TestResolveReasoningEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		effort string
		levels []string
		want   string
	}{
		{name: "empty effort", effort: "", levels: []string{"low", "medium", "high"}, want: ""},
		{name: "direct match", effort: "medium", levels: []string{"low", "medium", "high"}, want: "medium"},
		{name: "xhigh falls back to high", effort: "xhigh", levels: []string{"low", "medium", "high"}, want: "high"},
		{name: "max falls back to high", effort: "max", levels: []string{"low", "medium", "high"}, want: "high"},
		{name: "minimal falls back to low", effort: "minimal", levels: []string{"low", "medium", "high"}, want: "low"},
		{name: "xhigh supported", effort: "xhigh", levels: []string{"low", "medium", "high", "xhigh"}, want: "xhigh"},
		{name: "off maps to none", effort: "off", levels: []string{"none", "low", "medium", "high"}, want: "none"},
		{name: "off unsupported returns empty", effort: "off", levels: []string{"low", "medium", "high"}, want: ""},
		{name: "no levels", effort: "high", levels: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ResolveReasoningEffort(tt.effort, tt.levels))
		})
	}
}

func TestResolveModelReasoningEffort(t *testing.T) {
	t.Parallel()

	p := ProviderConfig{
		ID:   "openai",
		Name: "OpenAI",
		Models: []catwalk.Model{
			{ID: "gpt-4o", ReasoningLevels: []string{"low", "medium", "high"}},
		},
	}

	require.Equal(t, "high", ResolveModelReasoningEffort(p, "gpt-4o", "xhigh"))
	require.Equal(t, "medium", ResolveModelReasoningEffort(p, "gpt-4o", "medium"))
	require.Equal(t, "", ResolveModelReasoningEffort(p, "gpt-4o", "off"))
	require.Equal(t, "", ResolveModelReasoningEffort(p, "gpt-4o", ""))
	require.Equal(t, "", ResolveModelReasoningEffort(p, "nonexistent", "high"))
}
