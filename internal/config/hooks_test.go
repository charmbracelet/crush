package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

func TestValidateHooksInvalidRegex(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Hooks: map[string][]HookConfig{
			hooks.EventPreToolUse: {
				{Command: "true", Matcher: "[invalid"},
			},
		},
	}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid matcher regex")
}

func TestValidateHooksEmptyCommand(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Hooks: map[string][]HookConfig{
			hooks.EventPreToolUse: {
				{Command: ""},
			},
		},
	}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "command is required")
}

func TestValidateHooksNormalizesEventNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"canonical", "PreToolUse"},
		{"lowercase", "pretooluse"},
		{"snake_case", "pre_tool_use"},
		{"upper_snake", "PRE_TOOL_USE"},
		{"mixed_case", "preToolUse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{
				Hooks: map[string][]HookConfig{
					tt.input: {
						{Command: "true"},
					},
				},
			}
			require.NoError(t, cfg.ValidateHooks())
			require.Len(t, cfg.Hooks[hooks.EventPreToolUse], 1)
		})
	}
}

func TestValidateHooksRejectsUnknownEventAndLifecycleMatcher(t *testing.T) {
	t.Parallel()

	cfg := &Config{Hooks: map[string][]HookConfig{"TurnEnds": {{Command: "true"}}}}
	err := cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown event")

	cfg = &Config{Hooks: map[string][]HookConfig{hooks.EventTurnEnd: {{Command: "true", Matcher: "^bash$"}}}}
	err = cfg.ValidateHooks()
	require.Error(t, err)
	require.Contains(t, err.Error(), "matcher is only supported on tool events")

	cfg = &Config{Hooks: map[string][]HookConfig{hooks.EventPermissionRequest: {{Command: "true", Matcher: "^bash$"}}}}
	require.NoError(t, cfg.ValidateHooks())
}
