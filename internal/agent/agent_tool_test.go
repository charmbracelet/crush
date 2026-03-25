package agent

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubagentConfigUsesCanonicalExplore(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg}

	agentCfg, err := coord.subagentConfig(config.AgentTask)
	require.NoError(t, err)
	assert.Equal(t, config.AgentExplore, agentCfg.ID)

	agentCfg, err = coord.subagentConfig("")
	require.NoError(t, err)
	assert.Equal(t, config.AgentExplore, agentCfg.ID)
}

func TestSubagentConfigSupportsConfiguredSubagents(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	cfg.Config().Agents = map[string]config.Agent{
		"reviewer": {
			Mode:         config.AgentModeSubagent,
			Description:  "Reviews changes before handoff.",
			AllowedTools: []string{"view"},
		},
	}
	cfg.SetupAgents()

	coord := &coordinator{cfg: cfg}

	agentCfg, err := coord.subagentConfig("reviewer")
	require.NoError(t, err)
	assert.Equal(t, "reviewer", agentCfg.ID)
	assert.Equal(t, []string{"view"}, agentCfg.AllowedTools)
}

func TestBuildAgentToolDescriptionDeduplicatesExploreAlias(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg}
	description := coord.buildAgentToolDescription()

	assert.Contains(t, description, "- general:")
	assert.Contains(t, description, "- explore:")
	assert.Equal(t, 1, strings.Count(description, "- explore:"))
}

func TestBuildAgentToolDescriptionEmphasizesParallelDelegation(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg}
	description := coord.buildAgentToolDescription()

	assert.Contains(t, description, "If 2 or more substantial independent tasks can proceed in parallel")
	assert.Contains(t, description, "launch multiple Agent tool calls in the same assistant message")
	assert.Contains(t, description, "Prefer early delegation for bounded work")
	assert.Contains(t, description, "restricted `bash` tool")
	assert.Contains(t, description, "git diff")
	assert.Contains(t, description, "Do not claim that you are delegating")
	assert.Contains(t, description, "make the tool call first rather than narrating a future intention to delegate")
	assert.Contains(t, description, "Do not use the main thread for broad implementation work just because you already know which files are involved")
	assert.Contains(t, description, "prefer multiple direct tool calls in one response instead of subagents")
}

func TestCoderPromptTemplateRequiresOrchestrationFirstDelegation(t *testing.T) {
	promptText := string(coderPromptTmpl)

	assert.Contains(t, promptText, "The main agent is the orchestrator, not the default worker")
	assert.Contains(t, promptText, "you MUST prefer launching multiple Agent tool calls in the same assistant message")
	assert.Contains(t, promptText, "After delegating independent work, continue on the critical path locally")
	assert.Contains(t, promptText, "prefer batching direct tool calls in parallel instead of paying subagent overhead")
	assert.Contains(t, promptText, "Use subagents when each independent workstream is substantial enough")
	assert.Contains(t, promptText, "Do not merely say that you will use subagents or parallelize work")
	assert.Contains(t, promptText, "If you describe a plan that depends on subagents but then continue doing the delegated work yourself without calling `agent`, you are behaving incorrectly")
}

func TestBuildToolsForSubagentsUseExpectedCapabilities(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		userInput:   nil,
		history:     env.history,
		filetracker: *env.filetracker,
		lspManager:  lsp.NewManager(cfg),
	}

	generalTools, err := coord.buildTools(t.Context(), cfg.Config().Agents[config.AgentGeneral], session.CollaborationModeDefault)
	require.NoError(t, err)

	generalNames := make([]string, 0, len(generalTools))
	for _, tool := range generalTools {
		generalNames = append(generalNames, tool.Info().Name)
	}
	assert.Contains(t, generalNames, "bash")
	assert.Contains(t, generalNames, "edit")
	assert.NotContains(t, generalNames, AgentToolName)
	assert.NotContains(t, generalNames, "request_user_input")

	exploreTools, err := coord.buildTools(t.Context(), cfg.Config().Agents[config.AgentExplore], session.CollaborationModeDefault)
	require.NoError(t, err)

	exploreNames := make([]string, 0, len(exploreTools))
	for _, tool := range exploreTools {
		exploreNames = append(exploreNames, tool.Info().Name)
	}
	assert.Equal(t, []string{"bash", "glob", "grep", "ls", "sourcegraph", "view"}, exploreNames)
}

func TestBuildToolsForPlanModeUsesReadOnlyCapabilities(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		userInput:   nil,
		history:     env.history,
		filetracker: *env.filetracker,
		lspManager:  lsp.NewManager(cfg),
	}

	planTools, err := coord.buildTools(t.Context(), cfg.Config().Agents[config.AgentCoder], session.CollaborationModePlan)
	require.NoError(t, err)

	planNames := make([]string, 0, len(planTools))
	for _, tool := range planTools {
		planNames = append(planNames, tool.Info().Name)
	}

	assert.Equal(t, []string{
		"request_user_input",
		"plan_exit",
		"bash",
		"fetch",
		"glob",
		"grep",
		"ls",
		"sourcegraph",
		"view",
		"lsp_diagnostics",
		"lsp_references",
		"list_mcp_resources",
		"read_mcp_resource",
	}, planNames)
	assert.NotContains(t, planNames, AgentToolName)
	assert.NotContains(t, planNames, "edit")
	assert.NotContains(t, planNames, "multiedit")
	assert.NotContains(t, planNames, "write")
	assert.NotContains(t, planNames, "todos")
}
