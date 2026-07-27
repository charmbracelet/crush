package agent

import (
	"errors"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/workflow"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// newModelPinningCoordinator builds a coordinator whose large and small model
// selections are DIFFERENT models, so a test can tell which one an agent
// actually ended up running on. The provider constructs offline; no request is
// ever issued because these tests only inspect the built agent.
func newModelPinningCoordinator(t *testing.T) (*coordinator, string, string) {
	t.Helper()

	const (
		providerID   = "test-openai-compat"
		largeModelID = "test-model-large"
		smallModelID = "test-model-small"
	)

	workingDir := t.TempDir()
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	cfg.Config().Providers.Set(providerID, config.ProviderConfig{
		ID:      providerID,
		Name:    "Test",
		Type:    openaicompat.Name,
		BaseURL: "http://127.0.0.1:0/v1",
		APIKey:  "test",
		Models: []catwalk.Model{
			{ID: largeModelID, DefaultMaxTokens: 4096},
			{ID: smallModelID, DefaultMaxTokens: 4096},
		},
	})
	cfg.OverridePreferredModel(config.SelectedModelTypeLarge, config.SelectedModel{
		Provider: providerID, Model: largeModelID,
	})
	cfg.OverridePreferredModel(config.SelectedModelTypeSmall, config.SelectedModel{
		Provider: providerID, Model: smallModelID,
	})
	cfg.SetupAgents()

	return &coordinator{
		cfg:         cfg,
		permissions: permission.NewPermissionService(workingDir, true, nil),
	}, largeModelID, smallModelID
}

// TestBuildAgentHonoursPinnedModel is the regression test for config.Agent.Model
// being declared but never read: every agent ran on the large model, so a
// workflow sub-agent asking for the small model silently billed the large one.
func TestBuildAgentHonoursPinnedModel(t *testing.T) {
	// Deliberately not t.Parallel(): the subtests share one coordinator and
	// build real agents off it, and -race cannot run on every dev host, so
	// parallelising them would be an unverifiable change. tparallel then
	// requires the subtests to stay serial too.

	c, largeModelID, smallModelID := newModelPinningCoordinator(t)
	prmpt, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	require.NoError(t, err)

	for _, tc := range []struct {
		name  string
		pin   config.SelectedModelType
		wantK string
	}{
		{"pinned large", config.SelectedModelTypeLarge, largeModelID},
		{"pinned small", config.SelectedModelTypeSmall, smallModelID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := c.cfg.Config().Agents[config.AgentTask]
			cfg.Model = tc.pin

			agent, err := c.buildAgent(t.Context(), prmpt, cfg, true)
			require.NoError(t, err)
			require.Equal(t, tc.wantK, agent.Model().ModelCfg.Model,
				"agent pinned to %q must run on that model", tc.pin)
		})
	}
}

// TestWorkflowAgentsSelectsByProfileAndModel asserts the workflow tool resolves
// a spawn's profile AND model into the right agent config. Before the fix,
// SpawnOpts.Model was parsed and validated by the engine and then had no reader
// at all in workflow_tool.go, so model="small" was accepted and ignored.
func TestWorkflowAgentsSelectsByProfileAndModel(t *testing.T) {
	// Deliberately not t.Parallel(): the subtests share one coordinator and
	// build real agents off it, and -race cannot run on every dev host, so
	// parallelising them would be an unverifiable change. tparallel then
	// requires the subtests to stay serial too.

	c, largeModelID, smallModelID := newModelPinningCoordinator(t)
	// The same factory workflowTool installs, so the assertion covers the real
	// profile/model resolution rather than a copy of it.
	agents, err := c.newWorkflowAgents(t.Context())
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		key       workflowAgentKey
		wantModel string
	}{
		{"task inherits profile default", workflowAgentKey{profile: workflowProfileTask}, largeModelID},
		{"task on small", workflowAgentKey{profile: workflowProfileTask, model: "small"}, smallModelID},
		{"task on large", workflowAgentKey{profile: workflowProfileTask, model: "large"}, largeModelID},
		{"coder on small", workflowAgentKey{profile: workflowProfileCoder, model: "small"}, smallModelID},
		{"coder inherits profile default", workflowAgentKey{profile: workflowProfileCoder}, largeModelID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent, err := agents.get(tc.key)
			require.NoError(t, err)
			require.Equal(t, tc.wantModel, agent.Model().ModelCfg.Model,
				"profile=%q model=%q resolved to the wrong model", tc.key.profile, tc.key.model)
		})
	}
}

// TestWorkflowAgentsCachesPerKey asserts each distinct profile/model pair is
// built exactly once and shared, and that distinct pairs are not conflated.
func TestWorkflowAgentsCachesPerKey(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	builds := map[workflowAgentKey]int{}

	agents := &workflowAgents{
		cache: make(map[workflowAgentKey]SessionAgent),
		build: func(key workflowAgentKey) (SessionAgent, error) {
			mu.Lock()
			builds[key]++
			mu.Unlock()
			return &sessionAgent{}, nil
		},
	}

	small := workflowAgentKey{profile: workflowProfileTask, model: "small"}
	large := workflowAgentKey{profile: workflowProfileTask, model: "large"}

	first, err := agents.get(small)
	require.NoError(t, err)
	second, err := agents.get(small)
	require.NoError(t, err)
	require.Same(t, first, second, "the same key must reuse one agent")

	other, err := agents.get(large)
	require.NoError(t, err)
	require.NotSame(t, first, other, "different keys must not share an agent")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, builds[small], "small must be built exactly once")
	require.Equal(t, 1, builds[large], "large must be built exactly once")
}

// TestWorkflowAgentsBuildErrorPropagates makes sure a failed build is surfaced
// to the spawn rather than cached as a nil agent.
func TestWorkflowAgentsBuildErrorPropagates(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	agents := &workflowAgents{
		cache: make(map[workflowAgentKey]SessionAgent),
		build: func(workflowAgentKey) (SessionAgent, error) { return nil, sentinel },
	}

	got, err := agents.get(workflowAgentKey{profile: workflowProfileCoder})
	require.ErrorIs(t, err, sentinel)
	require.Nil(t, got)
	require.Empty(t, agents.cache, "a failed build must not be cached")
}

// TestWorkflowSpawnKeyCarriesModel guards the line that was actually broken:
// spawn resolved only opts.Agent, so opts.Model was parsed, validated by the
// engine, and then dropped on the floor. Every model="small" sub-agent silently
// ran on the large model.
func TestWorkflowSpawnKeyCarriesModel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		opts workflow.SpawnOpts
		want workflowAgentKey
	}{
		{
			"empty defaults to task, inherits profile model",
			workflow.SpawnOpts{},
			workflowAgentKey{profile: workflowProfileTask, model: ""},
		},
		{
			"model alone still defaults the profile",
			workflow.SpawnOpts{Model: "small"},
			workflowAgentKey{profile: workflowProfileTask, model: "small"},
		},
		{
			"coder on the small model",
			workflow.SpawnOpts{Agent: workflowProfileCoder, Model: "small"},
			workflowAgentKey{profile: workflowProfileCoder, model: "small"},
		},
		{
			"coder on the large model",
			workflow.SpawnOpts{Agent: workflowProfileCoder, Model: "large"},
			workflowAgentKey{profile: workflowProfileCoder, model: "large"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, workflowSpawnKey(tc.opts))
		})
	}
}
