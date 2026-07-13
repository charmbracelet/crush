package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAgentModeFromCommandIsExact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "/plan", want: config.AgentReview, ok: true},
		{input: " /plan ", want: config.AgentReview, ok: true},
		{input: "/build", want: config.AgentCoder, ok: true},
		{input: "/coder", want: config.AgentCoder, ok: true},
		{input: "/chat", want: config.AgentCoder, ok: true},
		{input: "/normal", want: config.AgentCoder, ok: true},
		{input: "/task", want: config.AgentCoder, ok: true},
		{input: "/goal", want: config.AgentGoal, ok: true},
		{input: "/review", want: config.AgentReview, ok: true},
		{input: "plan", ok: false},
		{input: "/plan this change", ok: false},
		{input: "please /plan", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := agentModeFromCommand(tt.input)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNextAgentModeCyclesTaskGoalAndReview(t *testing.T) {
	t.Parallel()

	require.Equal(t, config.AgentGoal, nextAgentMode(config.AgentCoder))
	require.Equal(t, config.AgentCoder, nextAgentMode(config.AgentPlan))
	require.Equal(t, config.AgentCoder, nextAgentMode(config.AgentTask))
	require.Equal(t, config.AgentReview, nextAgentMode(config.AgentGoal))
	require.Equal(t, config.AgentCoder, nextAgentMode(config.AgentReview))
	require.Equal(t, config.AgentGoal, nextAgentMode(""))
}

func TestStatusModeLabelMakesReadOnlyModeExplicit(t *testing.T) {
	t.Parallel()

	require.Equal(t, "MODE: TASK", statusModeLabel(config.AgentCoder))
	require.Equal(t, "MODE: TASK", statusModeLabel(config.AgentTask))
	require.Equal(t, "MODE: GOAL", statusModeLabel(config.AgentGoal))
	require.Equal(t, "MODE: REVIEW READ ONLY", statusModeLabel(config.AgentReview))
	require.Equal(t, "MODE: REVIEW READ ONLY", statusModeLabel(config.AgentPlan))
}

func TestHelpWithModePrefixAlignsEveryLine(t *testing.T) {
	t.Parallel()

	got := helpWithModePrefix("MODE: TASK", "tab  focus chat\nctrl+p  commands")
	require.Equal(t, "MODE: TASK | tab  focus chat\n             ctrl+p  commands", got)
}
