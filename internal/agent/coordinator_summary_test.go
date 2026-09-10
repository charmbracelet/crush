package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

// newSummaryTestCoordinator builds a coordinator with a summary model
// slot configured, for testing summary model resolution.
func newSummaryTestCoordinator(t *testing.T, crushJSON string) *coordinator {
	t.Helper()

	env := testEnv(t)

	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:          cfg,
		sessions:     env.sessions,
		messages:     env.messages,
		permissions:  env.permissions,
		history:      env.history,
		filetracker:  *env.filetracker,
		agents:       make(map[string]SessionAgent),
		summaryModel: csync.NewValue(Model{}),
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	agent, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.currentAgent = agent
	coord.agents[config.AgentCoder] = agent

	return coord
}

func TestBuildAgentInitializesSummaryModel(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"},
             "summary": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	// summaryModel should be populated after buildAgent.
	sm := coord.summaryModel.Get()
	require.NotNil(t, sm.Model, "summary model must be initialized after buildAgent")
	require.Equal(t, "mock-model", sm.ModelCfg.Model)
}

func TestBuildAgentFallsBackToSmallWhenSummaryAbsent(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	// summaryModel should fall back to small when summary slot absent.
	sm := coord.summaryModel.Get()
	require.NotNil(t, sm.Model, "summary model must fall back to small")
	require.Equal(t, "mock-model", sm.ModelCfg.Model)
}

func TestUpdateSummaryModelResolvesFromConfig(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"},
             "summary": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	err := coord.UpdateSummaryModel(context.Background())
	require.NoError(t, err)
	sm := coord.summaryModel.Get()
	require.NotNil(t, sm.Model)
	require.Equal(t, "mock-model", sm.ModelCfg.Model)
}

func TestUpdateSummaryModelFallsBackToSmallWhenAbsent(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	err := coord.UpdateSummaryModel(context.Background())
	require.NoError(t, err)
	sm := coord.summaryModel.Get()
	require.NotNil(t, sm.Model)
}

func TestUpdateModelsResolvesSummaryModel(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"},
             "summary": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	err := coord.UpdateModels(context.Background())
	require.NoError(t, err)
	sm := coord.summaryModel.Get()
	require.NotNil(t, sm.Model)
	require.Equal(t, "mock-model", sm.ModelCfg.Model)
}

func TestBuildSelectedModelReturnsErrModelNotFoundInProvider(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	// Request a model that doesn't exist in the provider catalog.
	_, err := coord.buildSelectedModel(context.Background(), config.SelectedModel{
		Provider: "mock",
		Model:    "nonexistent-model",
	}, true)
	require.ErrorIs(t, err, errModelNotFoundInProvider)
}

func TestBuildSelectedModelReturnsErrModelProviderNotConfigured(t *testing.T) {
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	coord := newSummaryTestCoordinator(t, crushJSON)

	// Request a provider that doesn't exist.
	_, err := coord.buildSelectedModel(context.Background(), config.SelectedModel{
		Provider: "nonexistent-provider",
		Model:    "some-model",
	}, true)
	require.ErrorIs(t, err, errModelProviderNotConfigured)
}
