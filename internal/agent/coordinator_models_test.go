package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestBuildAgentModelsResolvesCredentialScopedCatalog is a regression test
// for the "fetched OAuth catalog models cannot be selected" bug: the model
// picker lists credential-scoped catalogs (CopilotModels), but
// buildAgentModels resolved the selected ID only against the static Models
// list, so picking a subscription-granted model failed with
// errLargeModelNotFound. Resolution now goes through Config.GetModel, which
// includes the credential-scoped catalogs.
func TestBuildAgentModelsResolvesCredentialScopedCatalog(t *testing.T) {
	env := testEnv(t)

	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "static-model", "name": "Static", "context_window": 8192, "default_max_tokens": 128}],
    "copilot_models": [{"id": "scoped-model", "name": "Scoped", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "scoped-model"},
             "small": {"provider": "mock", "model": "scoped-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}

	large, small, err := coord.buildAgentModels(context.Background(), false)
	require.NoError(t, err, "models from credential-scoped catalogs must be selectable")
	require.Equal(t, "scoped-model", large.CatwalkCfg.ID)
	require.Equal(t, "scoped-model", small.CatwalkCfg.ID)
}
