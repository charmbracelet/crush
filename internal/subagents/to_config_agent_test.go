package subagents

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestToConfigAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subagent Subagent
		base     config.Agent
		check    func(t *testing.T, result config.Agent)
	}{
		{
			name:     "no_restrictions",
			subagent: Subagent{Name: "my-agent", Description: "Does something."},
			base: config.Agent{
				AllowedTools: []string{"bash", "grep", "view"},
				Model:        config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.Equal(t, []string{"bash", "grep", "view"}, result.AllowedTools)
			},
		},
		{
			name: "tools_filter",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
				Tools:       ToolList{"grep", "view"},
			},
			base: config.Agent{
				AllowedTools: []string{"bash", "grep", "view", "edit"},
				Model:        config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.ElementsMatch(t, []string{"grep", "view"}, result.AllowedTools)
			},
		},
		{
			name: "disallowed_tools",
			subagent: Subagent{
				Name:            "my-agent",
				Description:     "Does something.",
				DisallowedTools: ToolList{"view"},
			},
			base: config.Agent{
				AllowedTools: []string{"bash", "grep", "view"},
				Model:        config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.ElementsMatch(t, []string{"bash", "grep"}, result.AllowedTools)
			},
		},
		{
			name: "both_filters_disallowed_first",
			subagent: Subagent{
				Name:            "my-agent",
				Description:     "Does something.",
				DisallowedTools: ToolList{"bash"},
				Tools:           ToolList{"grep", "bash"},
			},
			base: config.Agent{
				AllowedTools: []string{"bash", "grep", "view"},
				Model:        config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				// disallowed removes "bash" first → base becomes ["grep","view"]
				// then tools filter intersects with ["grep","bash"] → only "grep" survives
				require.ElementsMatch(t, []string{"grep"}, result.AllowedTools)
			},
		},
		{
			name: "mcp_servers",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
				MCPServers:  []string{"github", "linear"},
			},
			base: config.Agent{
				Model: config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.NotNil(t, result.AllowedMCP)
				require.Contains(t, result.AllowedMCP, "github")
				require.Contains(t, result.AllowedMCP, "linear")
				// values should be nil (all tools from that MCP allowed)
				require.Nil(t, result.AllowedMCP["github"])
				require.Nil(t, result.AllowedMCP["linear"])
			},
		},
		{
			name: "mcp_servers_empty",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
				MCPServers:  nil,
			},
			base: config.Agent{
				Model: config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.Nil(t, result.AllowedMCP)
			},
		},
		{
			name: "model_small",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
				Model:       "small",
			},
			base: config.Agent{
				Model: config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.Equal(t, config.SelectedModelType("small"), result.Model)
			},
		},
		{
			name: "model_large",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
				Model:       "large",
			},
			base: config.Agent{
				Model: config.SelectedModelTypeSmall,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.Equal(t, config.SelectedModelType("large"), result.Model)
			},
		},
		{
			name: "model_empty",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
				Model:       "",
			},
			base: config.Agent{
				Model: config.SelectedModelTypeLarge,
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.Equal(t, config.SelectedModelTypeLarge, result.Model)
			},
		},
		{
			name: "id_and_name",
			subagent: Subagent{
				Name:        "my-agent",
				Description: "Does something.",
			},
			base: config.Agent{
				ID:   "old-id",
				Name: "old-name",
			},
			check: func(t *testing.T, result config.Agent) {
				t.Helper()
				require.Equal(t, "my-agent", result.ID)
				require.Equal(t, "my-agent", result.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.subagent.ToConfigAgent(tt.base)
			tt.check(t, result)
		})
	}
}
