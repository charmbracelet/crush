package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// newOfflineCoordinator builds a real coordinator against a local
// openai-compat provider config so no network call is needed, mirroring
// TestAgentTool_SubagentBuildFailure_SurfacedAsToolError.
func newOfflineCoordinator(t *testing.T, env fakeEnv) *coordinator {
	t.Helper()

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	const (
		providerID = "test-openai-compat"
		modelID    = "test-model"
	)
	cfg.Config().Providers.Set(providerID, config.ProviderConfig{
		ID:      providerID,
		Name:    "Test",
		Type:    openaicompat.Name,
		BaseURL: "http://127.0.0.1:0/v1",
		APIKey:  "test",
		Models:  []catwalk.Model{{ID: modelID, DefaultMaxTokens: 4096}},
	})
	selected := config.SelectedModel{Provider: providerID, Model: modelID}
	cfg.Config().Models[config.SelectedModelTypeLarge] = selected
	cfg.Config().Models[config.SelectedModelTypeSmall] = selected
	cfg.SetupAgents()

	// Clear AllowedTools on both agents so buildTools stays cheap and offline.
	for _, agentID := range []string{config.AgentCoder, config.AgentTask} {
		a := cfg.Config().Agents[agentID]
		a.AllowedTools = nil
		cfg.Config().Agents[agentID] = a
	}

	c, err := NewCoordinator(t.Context(), CoordinatorOptions{
		Config:      cfg,
		Sessions:    env.sessions,
		Messages:    env.messages,
		Permissions: permission.NewPermissionService(env.workingDir, true, nil),
	})
	require.NoError(t, err)
	return c.(*coordinator)
}

// TestBuildAgent_ProvidedGroupWaitMakesAgentReady verifies that buildAgent
// spawns its prompt and tool builds onto the caller-provided errgroup, so a
// dispatch-time caller that Waits on that group observes a fully-initialized
// agent (system prompt and tools set) before running it.
func TestBuildAgent_ProvidedGroupWaitMakesAgentReady(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newOfflineCoordinator(t, env)
	require.NoError(t, coord.readyWg.Wait())

	taskCfg := coord.cfg.Config().Agents[config.AgentTask]
	taskCfg.AllowedTools = []string{"glob"}

	taskPr, err := taskPrompt(prompt.WithWorkingDir(coord.cfg.WorkingDir()))
	require.NoError(t, err)

	var buildWg errgroup.Group
	built, err := coord.buildAgent(t.Context(), taskPr, taskCfg, true, subagentModel{}, &buildWg)
	require.NoError(t, err)
	require.NoError(t, buildWg.Wait())

	sa := built.(*sessionAgent)
	require.NotEmpty(t, sa.systemPrompt.Get(), "system prompt must be set once the provided group is waited on")
	require.Equal(t, 1, sa.tools.Len(), "tool set must be set once the provided group is waited on")
}

// TestBuildAgent_AsyncBuildFailureStaysOnProvidedGroup verifies that an
// asynchronous build failure surfaces on the caller-provided errgroup only and
// does not poison the coordinator-wide readyWg — whose error is sticky and
// would otherwise fail every subsequent coder turn (readyWg.Wait runs at the
// start of each run).
func TestBuildAgent_AsyncBuildFailureStaysOnProvidedGroup(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newOfflineCoordinator(t, env)
	require.NoError(t, coord.readyWg.Wait())

	taskCfg := coord.cfg.Config().Agents[config.AgentTask]

	// A template referencing a nonexistent field parses fine but fails at
	// Build (template execution), reproducing an async prompt-build failure.
	badPr, err := prompt.NewPrompt("bad", "{{.Config.NoSuchField}}")
	require.NoError(t, err)

	var buildWg errgroup.Group
	_, err = coord.buildAgent(t.Context(), badPr, taskCfg, true, subagentModel{}, &buildWg)
	require.NoError(t, err, "buildAgent itself must not fail; the failure is async")

	require.Error(t, buildWg.Wait(), "the provided group must carry the async build failure")
	require.NoError(t, coord.readyWg.Wait(), "the coordinator-wide readyWg must stay clean so later turns are unaffected")
}

// TestRefreshCoderSystemPrompt_TracksSubagentReloads verifies that
// refreshCoderSystemPrompt (called by UpdateModels at the start of every turn)
// rebuilds the coder system prompt when the active subagent set changes, so
// the <available_subagents> block tracks Library reloads instead of staying a
// construction-time snapshot — and that it skips the rebuild when nothing
// changed.
func TestRefreshCoderSystemPrompt_TracksSubagentReloads(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newOfflineCoordinator(t, env)
	require.NoError(t, coord.readyWg.Wait())

	sa := coord.currentAgent.(*sessionAgent)
	initial := sa.systemPrompt.Get()
	require.NotContains(t, initial, "<available_subagents>", "no subagents configured at construction")

	// A Library reload adds a subagent (activeSubagentsList falls back to
	// activeSubagents when no manager is wired).
	coord.activeSubagents = []*subagents.Subagent{
		{Name: "late-arrival", Description: "Added after construction."},
	}
	coord.refreshCoderSystemPrompt(t.Context(), coord.currentAgent.Model())

	refreshed := sa.systemPrompt.Get()
	require.Contains(t, refreshed, "<available_subagents>")
	require.Contains(t, refreshed, "<name>late-arrival</name>")

	// Unchanged set: the prompt must not be rebuilt (pointer-equal string
	// content is fine — assert stability instead of identity).
	coord.refreshCoderSystemPrompt(t.Context(), coord.currentAgent.Model())
	require.Equal(t, refreshed, sa.systemPrompt.Get())

	// Removing the subagent drops the block again.
	coord.activeSubagents = nil
	coord.refreshCoderSystemPrompt(t.Context(), coord.currentAgent.Model())
	require.NotContains(t, sa.systemPrompt.Get(), "<available_subagents>")
}
