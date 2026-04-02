package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
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
	assert.Contains(t, generalNames, tools.HistorySearchToolName)
	assert.Contains(t, generalNames, tools.LongTermMemoryToolName)
	assert.Contains(t, generalNames, tools.SendMessageToolName)
	assert.Contains(t, generalNames, tools.TaskStopToolName)
	assert.Contains(t, generalNames, tools.LSPCodeActionToolName)
	assert.Contains(t, generalNames, tools.LSPRenameToolName)
	assert.Contains(t, generalNames, tools.LSPFormatToolName)
	assert.NotContains(t, generalNames, AgentToolName)
	assert.NotContains(t, generalNames, "request_user_input")

	exploreTools, err := coord.buildTools(t.Context(), cfg.Config().Agents[config.AgentExplore], session.CollaborationModeDefault)
	require.NoError(t, err)

	exploreNames := make([]string, 0, len(exploreTools))
	for _, tool := range exploreTools {
		exploreNames = append(exploreNames, tool.Info().Name)
	}
	assert.Equal(t, []string{"bash", "glob", "grep", "ls", "tool_search", "view"}, exploreNames)
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
		"glob",
		"grep",
		"history_search",
		"ls",
		"lsp_declaration",
		"lsp_definition",
		"lsp_diagnostics",
		"lsp_document_symbols",
		"lsp_hover",
		"lsp_implementation",
		"lsp_references",
		"lsp_type_definition",
		"lsp_workspace_symbols",
		"plan_exit",
		"request_user_input",
		"view",
	}, planNames)
	assert.NotContains(t, planNames, AgentToolName)
	assert.NotContains(t, planNames, "agentic_fetch")
	assert.NotContains(t, planNames, "bash")
	assert.NotContains(t, planNames, "fetch")
	assert.NotContains(t, planNames, "sourcegraph")
	assert.NotContains(t, planNames, "tool_search")
	assert.NotContains(t, planNames, "list_mcp_resources")
	assert.NotContains(t, planNames, "read_mcp_resource")
	assert.NotContains(t, planNames, "edit")
	assert.NotContains(t, planNames, tools.LongTermMemoryToolName)
	assert.NotContains(t, planNames, "multiedit")
	assert.NotContains(t, planNames, "write")
	assert.NotContains(t, planNames, "todos")
}

func TestBuildToolsHonorsDisabledToolsInDefaultMode(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Options.DisabledTools = []string{"bash", "fetch"}

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

	defaultTools, err := coord.buildTools(t.Context(), cfg.Config().Agents[config.AgentCoder], session.CollaborationModeDefault)
	require.NoError(t, err)

	defaultNames := make([]string, 0, len(defaultTools))
	for _, tool := range defaultTools {
		defaultNames = append(defaultNames, tool.Info().Name)
	}

	assert.NotContains(t, defaultNames, "bash")
	assert.NotContains(t, defaultNames, "fetch")
	assert.Contains(t, defaultNames, "view")
	assert.Contains(t, defaultNames, "write")
}

func runAgentToolForTest(t *testing.T, tool fantasy.AgentTool, params AgentParams) (fantasy.ToolResponse, error) {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, "session-1")
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "msg-1")
	return tool.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: AgentToolName, Input: string(input)})
}

func TestAgentToolUsesTaskGraphWhenTasksProvided(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg}
	called := false
	coord.taskGraphScheduler = func(_ context.Context, params taskGraphParams) (fantasy.ToolResponse, error) {
		called = true
		require.Equal(t, "session-1", params.SessionID)
		require.Equal(t, "msg-1", params.AgentMessageID)
		require.Equal(t, "call-1", params.ToolCallID)
		require.Len(t, params.Tasks, 2)
		require.Equal(t, "fetch", params.Tasks[0].ID)
		require.Equal(t, []string{"fetch"}, params.Tasks[1].DependsOn)
		return fantasy.NewTextResponse("graph"), nil
	}

	tool, err := coord.agentTool(t.Context())
	require.NoError(t, err)

	resp, err := runAgentToolForTest(t, tool, AgentParams{Tasks: []AgentTaskParams{
		{ID: "fetch", Prompt: "fetch info", SubagentType: "explore", Description: "Fetch"},
		{ID: "summarize", Prompt: "summarize info", SubagentType: "general", DependsOn: []string{"fetch"}},
	}})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "graph", resp.Content)
}

func TestAgentToolKeepsSinglePromptPathCompatible(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg}
	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		return &mockSessionAgent{}, config.Agent{ID: config.CanonicalSubagentID(requestedType), Mode: config.AgentModeSubagent}, nil
	}

	called := false
	coord.taskGraphScheduler = func(_ context.Context, params taskGraphParams) (fantasy.ToolResponse, error) {
		called = true
		require.Equal(t, "call-1", params.ToolCallID)
		require.Len(t, params.Tasks, 1)
		require.Equal(t, "task", params.Tasks[0].ID)
		require.Equal(t, "fix issue", params.Tasks[0].Prompt)
		return fantasy.NewTextResponse("single"), nil
	}

	tool, err := coord.agentTool(t.Context())
	require.NoError(t, err)

	resp, err := runAgentToolForTest(t, tool, AgentParams{Prompt: "fix issue", SubagentType: "general"})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "single", resp.Content)
}
