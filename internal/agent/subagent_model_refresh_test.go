package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// authRecorder is a provider endpoint that records the Authorization header of
// every request and rejects it, which is enough to see which credential a
// model was built with.
type authRecorder struct {
	mu   sync.Mutex
	seen []string
}

func (r *authRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.seen = append(r.seen, req.Header.Get("Authorization"))
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"message":"rejected","type":"authentication_error"}}`))
}

// send makes one request through model and returns the credential it carried.
func (r *authRecorder) send(t *testing.T, model Model) string {
	t.Helper()
	maxTokens := int64(16)
	_, _ = model.Model.Generate(t.Context(), fantasy.Call{
		Prompt:          fantasy.Prompt{fantasy.NewUserMessage("hi")},
		MaxOutputTokens: &maxTokens,
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.seen)
	return r.seen[len(r.seen)-1]
}

func newPinnedModelCoordinator(t *testing.T, srvURL string) *coordinator {
	t.Helper()
	env := testEnv(t)
	crushJSON := fmt.Sprintf(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai-compat",
    "base_url": %q, "api_key": "first",
    "models": [
      {"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128},
      {"id": "pinned-model", "name": "Pinned", "context_window": 8192, "default_max_tokens": 128,
       "can_reason": true, "reasoning_levels": ["low", "medium", "high"], "default_reasoning_effort": "low"}
    ]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, srvURL)
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{
		cfg:                cfg,
		sessions:           env.sessions,
		messages:           env.messages,
		permissions:        env.permissions,
		history:            env.history,
		filetracker:        *env.filetracker,
		subagentModelCache: csync.NewMap[subagentModelKey, Model](),
	}
	require.NoError(t, coord.rebuildModels(t.Context()))
	return coord
}

// A subagent pinned to a specific model must follow a credential refresh like
// the agents on the shared models do, instead of keeping the credential its
// cached build was made with.
func TestPinnedSubagentModelFollowsCredentialRefresh(t *testing.T) {
	t.Parallel()

	rec := &authRecorder{}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	coord := newPinnedModelCoordinator(t, srv.URL)

	models, err := coord.subagentModels(t.Context(), coord.subAgentModels, subagentModel{Model: "pinned-model"}, true)
	require.NoError(t, err)
	require.Equal(t, "pinned-model", models().Large.ModelCfg.Model)
	require.Equal(t, "Bearer first", rec.send(t, models().Large))

	// Store the renewed credential, then rebuild the shared models, as a
	// refresh does.
	pc, ok := coord.cfg.Config().Providers.Get("mock")
	require.True(t, ok)
	pc.APIKey = "second"
	coord.cfg.Config().Providers.Set("mock", pc)
	require.NoError(t, coord.rebuildModels(t.Context()))

	require.Equal(t, "pinned-model", models().Large.ModelCfg.Model)
	require.Equal(t, "Bearer second", rec.send(t, models().Large))
}

// When a rebuild cannot resolve the pinned model any more, the agent keeps the
// last build that worked and the shared small model stays shared.
func TestPinnedSubagentModelKeepsLastBuildOnFailure(t *testing.T) {
	t.Parallel()

	rec := &authRecorder{}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	coord := newPinnedModelCoordinator(t, srv.URL)

	models, err := coord.subagentModels(t.Context(), coord.subAgentModels, subagentModel{Model: "pinned-model"}, true)
	require.NoError(t, err)

	pc, ok := coord.cfg.Config().Providers.Get("mock")
	require.True(t, ok)
	pc.Models = pc.Models[:1]
	coord.cfg.Config().Providers.Set("mock", pc)
	require.NoError(t, coord.rebuildModels(t.Context()))

	set := models()
	require.Equal(t, "pinned-model", set.Large.ModelCfg.Model, "the last good build is kept")
	require.Equal(t, "mock-model", set.Small.ModelCfg.Model)
}

// A built subagent keeps its model selection and effort, and reads the renewed
// credential, after a refresh rebuilds the shared models.
func TestBuiltSubagentKeepsSelectionAcrossCredentialRefresh(t *testing.T) {
	t.Parallel()

	rec := &authRecorder{}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	coord := newPinnedModelCoordinator(t, srv.URL)

	p, err := taskPrompt(prompt.WithWorkingDir(coord.cfg.WorkingDir()))
	require.NoError(t, err)
	var wg errgroup.Group
	sub, err := coord.buildAgent(t.Context(), p, coord.cfg.Config().Agents[config.AgentTask], true,
		subagentModel{Model: "pinned-model", Effort: "high"}, &wg)
	require.NoError(t, err)
	require.NoError(t, wg.Wait())

	before := sub.Model()
	require.Equal(t, "pinned-model", before.ModelCfg.Model)
	require.Equal(t, "high", before.ModelCfg.ReasoningEffort)
	require.Equal(t, "Bearer first", rec.send(t, before))

	pc, ok := coord.cfg.Config().Providers.Get("mock")
	require.True(t, ok)
	pc.APIKey = "second"
	coord.cfg.Config().Providers.Set("mock", pc)
	require.NoError(t, coord.rebuildModels(t.Context()))

	after := sub.Model()
	require.Equal(t, "pinned-model", after.ModelCfg.Model, "the pinned model survives the rebuild")
	require.Equal(t, "high", after.ModelCfg.ReasoningEffort, "the requested effort survives the rebuild")
	require.Equal(t, "Bearer second", rec.send(t, after))
}
