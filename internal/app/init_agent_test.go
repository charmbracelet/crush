package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// hermeticApp returns an App wired to a config with one provider pointed
// at a closed port, which is enough for the coordinator build to resolve
// models and build a system prompt without any network access.
func hermeticApp(t *testing.T) *App {
	t.Helper()

	dir := t.TempDir()
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(crushJSON), 0o644))

	store, err := config.Init(dir, "", false)
	require.NoError(t, err)
	store.SetupAgents()

	app := NewForTest(t.Context())
	t.Cleanup(app.ShutdownForTest)
	app.config = store
	// An empty manager, so the build does not scan the machine's skill
	// directories.
	app.Skills = skills.NewManager(nil, nil, nil)
	return app
}

// TestInitCoderAgentKeepsTheCoordinatorItAlreadyHas pins the fix for the
// runaway-session incident: /agent/init used to replace
// app.AgentCoordinator on every call, and every client attach and every
// client reconnect calls it.
//
// Everything that arbitrates a session lives on the coordinator
// instance, so a replacement left runs already in flight where cancel
// and busy state could not reach them, and let a new prompt start a
// second concurrent turn on a session that was already streaming.
func TestInitCoderAgentKeepsTheCoordinatorItAlreadyHas(t *testing.T) {
	t.Parallel()
	app := hermeticApp(t)

	require.NoError(t, app.InitCoderAgent(t.Context()))
	first := app.AgentCoordinator
	require.NotNil(t, first, "the first init must build a coordinator")

	require.NoError(t, app.InitCoderAgent(t.Context()))
	require.Same(t, first, app.AgentCoordinator,
		"a second init must keep the coordinator that owns the workspace's runs")
}

// TestInitCoderAgentConcurrentAttachesBuildOne pins the check-and-set:
// two clients attaching at the same time must not each build a
// coordinator, which would leave one of them serving runs nobody can
// cancel.
func TestInitCoderAgentConcurrentAttachesBuildOne(t *testing.T) {
	t.Parallel()
	app := hermeticApp(t)

	const attaches = 8
	start := make(chan struct{})
	errs := make(chan error, attaches)
	built := make(chan any, attaches)
	for range attaches {
		go func() {
			<-start
			errs <- app.InitCoderAgent(t.Context())
			built <- app.AgentCoordinator
		}()
	}
	close(start)

	for range attaches {
		require.NoError(t, <-errs)
	}
	first := <-built
	require.NotNil(t, first)
	for range attaches - 1 {
		require.Same(t, first, <-built, "concurrent attaches built more than one coordinator")
	}
}
