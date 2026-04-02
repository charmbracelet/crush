package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReadOnlyAgent(t *testing.T) {
	tests := []struct {
		name         string
		allowedTools []string
		expected     bool
	}{
		{
			name:         "nil allowed tools returns false",
			allowedTools: nil,
			expected:     false,
		},
		{
			name:         "empty allowed tools returns false",
			allowedTools: []string{},
			expected:     false,
		},
		{
			name:         "read-only tools returns true",
			allowedTools: []string{"glob", "grep", "ls", "view"},
			expected:     true,
		},
		{
			name:         "read-only tools with sourcegraph returns true",
			allowedTools: []string{"sourcegraph", "view", "grep"},
			expected:     true,
		},
		{
			name:         "mixed tools returns false",
			allowedTools: []string{"glob", "bash", "view"},
			expected:     false,
		},
		{
			name:         "write tools returns false",
			allowedTools: []string{"edit", "write"},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := config.Agent{
				AllowedTools: tt.allowedTools,
			}
			result := isReadOnlyAgent(agent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildSubagentPromptTemplateIncludesRoleAndAdditionalPrompt(t *testing.T) {
	t.Parallel()

	template := buildSubagentPromptTemplate("Base prompt.", config.Agent{
		Role:             "planner",
		AdditionalPrompt: "Return a concise plan.",
	})

	assert.Contains(t, template, "Base prompt.")
	assert.Contains(t, template, "You are running as a delegated subagent")
	assert.Contains(t, template, "Role: planner")
	assert.Contains(t, template, "Return a concise plan.")
}

func TestBuildSubagentRolePromptSupportsCommonRoles(t *testing.T) {
	t.Parallel()

	assert.Contains(t, buildSubagentRolePrompt(config.Agent{Role: "planner"}), "Role: planner")
	assert.Contains(t, buildSubagentRolePrompt(config.Agent{Role: "reviewer"}), "Role: reviewer")
	assert.Contains(t, buildSubagentRolePrompt(config.Agent{Role: "executor"}), "Role: executor")
	assert.Contains(t, buildSubagentRolePrompt(config.Agent{Role: "custom-role"}), "Role: custom-role")
	assert.Empty(t, buildSubagentRolePrompt(config.Agent{}))
}

func TestPromptForAgentBuildIncludesConfiguredRolePrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Options.ContextPaths = nil

	promptBuilder, err := promptForAgent(config.Agent{
		ID:               config.AgentGeneral,
		Role:             "executor",
		AdditionalPrompt: "Verify before handoff.",
		AllowedTools:     []string{"edit", "view"},
	}, true)
	require.NoError(t, err)

	built, err := promptBuilder.Build(t.Context(), "", "", cfg)
	require.NoError(t, err)
	assert.Contains(t, built, "Role: executor")
	assert.Contains(t, built, "Verify before handoff.")
}

func TestPromptForAgentBuildIncludesLifecyclePolicyAndInitialPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Options.ContextPaths = nil
	background := true

	promptBuilder, err := promptForAgent(config.Agent{
		ID:            config.AgentGeneral,
		Role:          "executor",
		InitialPrompt: "Complete the delegated task end-to-end.",
		Memory:        "isolated",
		Isolation:     "session",
		Background:    &background,
		AllowedTools:  []string{"view"},
	}, true)
	require.NoError(t, err)

	built, err := promptBuilder.Build(t.Context(), "", "", cfg)
	require.NoError(t, err)
	assert.Contains(t, built, "<agent_lifecycle>")
	assert.Contains(t, built, "background: true")
	assert.Contains(t, built, "memory: isolated")
	assert.Contains(t, built, "isolation: session")
	assert.Contains(t, built, "<initial_prompt>")
	assert.Contains(t, built, "Complete the delegated task end-to-end.")
}

func TestPromptForAgentOmitContextFilesSkipsProjectAndGlobalContext(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Options.ContextPaths = []string{env.workingDir}

	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "AGENTS.md"), []byte("PROJECT_CONTEXT"), 0o644))

	promptBuilder, err := promptForAgent(config.Agent{
		ID:               config.AgentGeneral,
		AllowedTools:     []string{"view"},
		OmitContextFiles: true,
	}, true)
	require.NoError(t, err)

	built, err := promptBuilder.Build(t.Context(), "", "", cfg)
	require.NoError(t, err)
	assert.NotContains(t, built, "PROJECT_CONTEXT")
}

func TestPromptForAgentContextPathsOverrideUsesAgentSpecificPaths(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	defaultDir := filepath.Join(env.workingDir, "default_ctx")
	overrideDir := filepath.Join(env.workingDir, "override_ctx")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	require.NoError(t, os.MkdirAll(overrideDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "AGENTS.md"), []byte("DEFAULT_CONTEXT"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "AGENTS.md"), []byte("OVERRIDE_CONTEXT"), 0o644))

	cfg.Config().Options.ContextPaths = []string{defaultDir}

	promptBuilder, err := promptForAgent(config.Agent{
		ID:           config.AgentGeneral,
		AllowedTools: []string{"view"},
		ContextPaths: []string{overrideDir},
	}, true)
	require.NoError(t, err)

	built, err := promptBuilder.Build(t.Context(), "", "", cfg)
	require.NoError(t, err)
	assert.NotContains(t, built, "DEFAULT_CONTEXT")
}
