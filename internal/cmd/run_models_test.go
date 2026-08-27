package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// modelServer serves the reads resolveModels and restoreModelFromSession
// need and fails the test on any write. `crush run -m` chooses a model
// for one run; writing it to the workspace would change the model of
// every other client and session in the directory and would outlive the
// command.
func modelServer(t *testing.T, cfg *config.Config, msgs []proto.Message, smallModel *config.SelectedModel) *client.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("choosing a model for one run must not write to the workspace: %s %s",
				r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			require.NoError(t, json.NewEncoder(w).Encode(cfg))
		case strings.HasSuffix(r.URL.Path, "/default-small-model"):
			require.NoError(t, json.NewEncoder(w).Encode(smallModel))
		case strings.HasSuffix(r.URL.Path, "/messages"):
			require.NoError(t, json.NewEncoder(w).Encode(msgs))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return c
}

// twoProviderConfig is a config with one model per provider, so a bare
// model name resolves unambiguously.
func twoProviderConfig() *config.Config {
	return &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "alpha", Model: "alpha-large"},
			config.SelectedModelTypeSmall: {Provider: "alpha", Model: "alpha-small"},
		},
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"alpha": {
				ID: "alpha",
				Models: []catwalk.Model{
					{ID: "alpha-large"},
					{ID: "alpha-small"},
				},
			},
			"beta": {
				ID: "beta",
				Models: []catwalk.Model{
					{ID: "beta-large"},
					{ID: "beta-small"},
				},
			},
		}),
	}
}

func TestResolveModelsDoesNotWriteWorkspaceConfig(t *testing.T) {
	t.Parallel()

	cfg := twoProviderConfig()
	betaSmall := &config.SelectedModel{Provider: "beta", Model: "beta-small"}
	c := modelServer(t, cfg, nil, betaSmall)
	ws := &proto.Workspace{ID: "ws1", Config: cfg}

	large, small, err := resolveModels(t.Context(), c, ws, "beta-large", "")
	require.NoError(t, err)

	require.NotNil(t, large)
	assert.Equal(t, "beta", large.Provider)
	assert.Equal(t, "beta-large", large.Model)

	// A large-only override must move the small model to the same
	// provider, or title generation and summarizing would stay behind on
	// the workspace's provider.
	require.NotNil(t, small)
	assert.Equal(t, *betaSmall, *small)
}

func TestResolveModelsResolvesBothSlots(t *testing.T) {
	t.Parallel()

	cfg := twoProviderConfig()
	c := modelServer(t, cfg, nil, nil)
	ws := &proto.Workspace{ID: "ws1", Config: cfg}

	large, small, err := resolveModels(t.Context(), c, ws, "beta/beta-large", "alpha/alpha-small")
	require.NoError(t, err)

	require.NotNil(t, large)
	assert.Equal(t, config.SelectedModel{Provider: "beta", Model: "beta-large"}, *large)
	require.NotNil(t, small)
	assert.Equal(t, config.SelectedModel{Provider: "alpha", Model: "alpha-small"}, *small)
}

func TestResolveModelsRejectsUnknownModel(t *testing.T) {
	t.Parallel()

	cfg := twoProviderConfig()
	c := modelServer(t, cfg, nil, nil)
	ws := &proto.Workspace{ID: "ws1", Config: cfg}

	_, _, err := resolveModels(t.Context(), c, ws, "nope", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRestoreModelFromSessionReturnsPairWithoutWriting(t *testing.T) {
	t.Parallel()

	cfg := twoProviderConfig()
	msgs := []proto.Message{
		{Role: proto.User},
		{Role: proto.Assistant, Provider: "beta", Model: "beta-large"},
	}
	c := modelServer(t, cfg, msgs, nil)
	ws := &proto.Workspace{ID: "ws1", Config: cfg}

	large, small, err := restoreModelFromSession(t.Context(), c, ws, "sess1")
	require.NoError(t, err)

	require.NotNil(t, large)
	assert.Equal(t, config.SelectedModel{Provider: "beta", Model: "beta-large"}, *large)
	// The workspace already has a small model, so continuing the session
	// leaves that slot alone.
	assert.Nil(t, small)
}

func TestRestoreModelFromSessionSkipsUnavailableModel(t *testing.T) {
	t.Parallel()

	cfg := twoProviderConfig()
	msgs := []proto.Message{
		{Role: proto.Assistant, Provider: "gamma", Model: "gone"},
	}
	c := modelServer(t, cfg, msgs, nil)
	ws := &proto.Workspace{ID: "ws1", Config: cfg}

	large, small, err := restoreModelFromSession(t.Context(), c, ws, "sess1")
	require.NoError(t, err)
	assert.Nil(t, large)
	assert.Nil(t, small)
}
