package crush

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	t.Parallel()

	// Happy path with defaults.
	a, err := NewAgent("coder")
	require.NoError(t, err)
	assert.Equal(t, "coder", a.ID)
	assert.Equal(t, SelectedModelTypeLarge, a.Model)
	assert.Nil(t, a.AllowedTools)
	assert.Nil(t, a.AllowedMCP)
	assert.False(t, a.Disabled)

	// Full options.
	a, err = NewAgent("task",
		WithAgentName("Task Agent"),
		WithAgentDescription("Searches for context."),
		WithAgentModel(SelectedModelTypeSmall),
		WithAgentAllowedTools("grep", "ls"),
		WithAgentContextPaths("README.md"),
		WithAgentDisabled(true),
	)
	require.NoError(t, err)
	assert.Equal(t, "task", a.ID)
	assert.Equal(t, "Task Agent", a.Name)
	assert.Equal(t, "Searches for context.", a.Description)
	assert.Equal(t, SelectedModelTypeSmall, a.Model)
	assert.Equal(t, []string{"grep", "ls"}, a.AllowedTools)
	assert.Equal(t, []string{"README.md"}, a.ContextPaths)
	assert.True(t, a.Disabled)
}

func TestNewAgent_EmptyID(t *testing.T) {
	t.Parallel()

	_, err := NewAgent("")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAgentIDRequired)

	_, err = NewAgent("   ")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAgentIDRequired)
}

func TestNewAgent_InvalidModel(t *testing.T) {
	t.Parallel()

	_, err := NewAgent("coder", WithAgentModel("fast"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAgentModelInvalid)
}

func TestNewAgent_AllowedMCP(t *testing.T) {
	t.Parallel()

	// With explicit tools.
	a, err := NewAgent("coder",
		WithAgentAllowedMCP("server1", "tool1", "tool2"),
	)
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"server1": {"tool1", "tool2"}}, a.AllowedMCP)

	// With no tools (nil slice means all tools for that MCP).
	a, err = NewAgent("coder",
		WithAgentAllowedMCP("server1"),
	)
	require.NoError(t, err)
	assert.Nil(t, a.AllowedMCP["server1"])

	// Multiple MCPs.
	a, err = NewAgent("coder",
		WithAgentAllowedMCP("a", "t1"),
		WithAgentAllowedMCP("b"),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"t1"}, a.AllowedMCP["a"])
	assert.Nil(t, a.AllowedMCP["b"])
}

func TestNewAgent_DisableMCPs(t *testing.T) {
	t.Parallel()

	a, err := NewAgent("task", WithAgentDisableMCPs())
	require.NoError(t, err)
	assert.NotNil(t, a.AllowedMCP)
	assert.Empty(t, a.AllowedMCP)
}

func TestNewAgent_AllowedMCP_LastWriteWins(t *testing.T) {
	t.Parallel()

	a, err := NewAgent("coder",
		WithAgentAllowedMCP("server1", "t1"),
		WithAgentAllowedMCP("server1", "t2"),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"t2"}, a.AllowedMCP["server1"])
}

func TestWithAgentDisabled(t *testing.T) {
	t.Parallel()

	a, err := NewAgent("coder", WithAgentDisabled(true))
	require.NoError(t, err)
	assert.True(t, a.Disabled)
}

func TestValidateAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		agent   Agent
		wantErr error
	}{
		{
			name: "valid with large model",
			agent: Agent{
				ID:    "coder",
				Model: SelectedModelTypeLarge,
			},
		},
		{
			name: "valid with small model",
			agent: Agent{
				ID:    "task",
				Model: SelectedModelTypeSmall,
			},
		},
		{
			name: "missing id",
			agent: Agent{
				ID:    "",
				Model: SelectedModelTypeLarge,
			},
			wantErr: ErrAgentIDRequired,
		},
		{
			name: "whitespace id",
			agent: Agent{
				ID:    "   ",
				Model: SelectedModelTypeLarge,
			},
			wantErr: ErrAgentIDRequired,
		},
		{
			name: "invalid model",
			agent: Agent{
				ID:    "coder",
				Model: "medium",
			},
			wantErr: ErrAgentModelInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateAgent(tt.agent)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAgentIsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, IsAgentValid(Agent{ID: "a", Model: SelectedModelTypeLarge}))
	assert.False(t, IsAgentValid(Agent{ID: ""}))
	assert.False(t, IsAgentValid(Agent{ID: "a", Model: "unknown"}))
}

func TestCloneAgent(t *testing.T) {
	t.Parallel()

	original := &Agent{
		ID:           "coder",
		Name:         "Coder",
		Description:  "A coding agent.",
		Disabled:     true,
		Model:        SelectedModelTypeLarge,
		AllowedTools: []string{"edit", "bash"},
		AllowedMCP: map[string][]string{
			"server1": {"tool1", "tool2"},
			"server2": nil,
		},
		ContextPaths: []string{"AGENTS.md"},
	}

	cloned := CloneAgent(*original)
	assert.EqualValues(t, *original, cloned)

	// Mutate slices and maps on the clone to verify deep copy.
	cloned.AllowedTools[0] = "modified"
	cloned.AllowedMCP["server1"][0] = "modified"
	cloned.AllowedMCP["new"] = []string{"tool"}
	cloned.ContextPaths[0] = "modified"

	assert.Equal(t, "edit", original.AllowedTools[0])
	assert.Equal(t, "tool1", original.AllowedMCP["server1"][0])
	assert.NotContains(t, original.AllowedMCP, "new")
	assert.Equal(t, "AGENTS.md", original.ContextPaths[0])
}

func TestCloneAgent_ZeroValue(t *testing.T) {
	t.Parallel()

	var a Agent
	cloned := CloneAgent(a)
	assert.Equal(t, a, cloned)
}
