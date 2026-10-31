package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestBuildAgentModelsResolvesCredentialScopedCatalog proves models from a
// subscription catalog, not just the static one, can be selected.
func TestBuildAgentModelsResolvesCredentialScopedCatalog(t *testing.T) {
	env := testEnv(t)

	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "static-model", "name": "Static", "context_window": 8192, "default_max_tokens": 128}],
    "subscription_models": [{"id": "scoped-model", "name": "Scoped", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "scoped-model"},
             "small": {"provider": "mock", "model": "scoped-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
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

	require.NoError(t, coord.rebuildModels(t.Context()))
	models := coord.mainModels()
	require.Equal(t, "scoped-model", models.Large.CatwalkCfg.ID, "models from credential-scoped catalogs must be selectable")
	require.Equal(t, "scoped-model", models.Small.CatwalkCfg.ID)
}
