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
		{input: "/plan", want: config.AgentPlan, ok: true},
		{input: " /plan ", want: config.AgentPlan, ok: true},
		{input: "/build", want: config.AgentCoder, ok: true},
		{input: "/coder", want: config.AgentCoder, ok: true},
		{input: "/task", want: config.AgentTask, ok: true},
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

func TestNextAgentModeCyclesBuildPlanTaskReview(t *testing.T) {
	t.Parallel()

	require.Equal(t, config.AgentPlan, nextAgentMode(config.AgentCoder))
	require.Equal(t, config.AgentTask, nextAgentMode(config.AgentPlan))
	require.Equal(t, config.AgentReview, nextAgentMode(config.AgentTask))
	require.Equal(t, config.AgentCoder, nextAgentMode(config.AgentReview))
	require.Equal(t, config.AgentPlan, nextAgentMode(""))
}
