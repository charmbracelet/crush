package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestAgentModelInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params agent.AgentParams
		want   string
	}{
		{
			name:   "default model shows nothing",
			params: agent.AgentParams{Prompt: "task"},
			want:   "",
		},
		{
			name:   "provider and model",
			params: agent.AgentParams{Provider: "anthropic", Model: "claude-sonnet-4"},
			want:   "anthropic/claude-sonnet-4",
		},
		{
			name:   "model only",
			params: agent.AgentParams{Model: "claude-sonnet-4"},
			want:   "claude-sonnet-4",
		},
		{
			name:   "provider alone shows nothing",
			params: agent.AgentParams{Provider: "anthropic"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, agentModelInfo(tt.params))
		})
	}
}

func TestAgentMetaLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params agent.AgentParams
		want   string
	}{
		{
			name:   "defaults show nothing",
			params: agent.AgentParams{Prompt: "task"},
			want:   "",
		},
		{
			name:   "write tools only",
			params: agent.AgentParams{WriteTools: true},
			want:   "(write_tools=true)",
		},
		{
			name:   "model only",
			params: agent.AgentParams{Model: "claude-sonnet-4"},
			want:   "(model=claude-sonnet-4)",
		},
		{
			name:   "all non-default params",
			params: agent.AgentParams{WriteTools: true, Provider: "openai", Model: "gpt-6-luna"},
			want:   "(model=openai/gpt-6-luna, write_tools=true)",
		},
		{
			name:   "provider alone shows nothing",
			params: agent.AgentParams{Provider: "openai"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, agentMetaLine(tt.params))
		})
	}
}
