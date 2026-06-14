package model

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestCurrentModelSupportsImages(t *testing.T) {
	t.Parallel()

	t.Run("returns false when config is nil", func(t *testing.T) {
		t.Parallel()

		ui := newTestUIWithConfig(t, nil)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when coder agent is missing", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents:    map[string]config.Agent{},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when model is not found", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns true when current model supports images", func(t *testing.T) {
		t.Parallel()

		providers := csync.NewMap[string, config.ProviderConfig]()
		providers.Set("test-provider", config.ProviderConfig{
			ID: "test-provider",
			Models: []catwalk.Model{
				{ID: "test-model", SupportsImages: true},
			},
		})

		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {
					Provider: "test-provider",
					Model:    "test-model",
				},
			},
			Providers: providers,
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}

		ui := newTestUIWithConfig(t, cfg)
		require.True(t, ui.currentModelSupportsImages())
	})
}

func TestHandleSelectModelPersistsWorkspaceScope(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("test-provider", config.ProviderConfig{
		ID:     "test-provider",
		APIKey: "test-key",
		Models: []catwalk.Model{{ID: "large-model", Name: "Large Model"}},
	})
	cfg := &config.Config{
		Models:    map[config.SelectedModelType]config.SelectedModel{},
		Providers: providers,
		Options:   &config.Options{TUI: &config.TUIOptions{}},
	}
	ws := &testWorkspace{
		cfg: cfg,
		defaultSmallModel: config.SelectedModel{
			Provider: "test-provider",
			Model:    "small-model",
		},
	}
	ui := New(&common.Common{Workspace: ws, Styles: ptr(styles.CharmtonePantera())}, "", false)

	ui.handleSelectModel(dialog.ActionSelectModel{
		Provider: catwalk.Provider{ID: "test-provider"},
		Model: config.SelectedModel{
			Provider: "test-provider",
			Model:    "large-model",
		},
		ModelType: config.SelectedModelTypeLarge,
	})

	require.Len(t, ws.preferredModelUpdates, 2)
	require.Equal(t, config.ScopeWorkspace, ws.preferredModelUpdates[0].scope)
	require.Equal(t, config.SelectedModelTypeLarge, ws.preferredModelUpdates[0].modelType)
	require.Equal(t, "large-model", ws.preferredModelUpdates[0].model.Model)
	require.Equal(t, config.ScopeWorkspace, ws.preferredModelUpdates[1].scope)
	require.Equal(t, config.SelectedModelTypeSmall, ws.preferredModelUpdates[1].modelType)
	require.Equal(t, "small-model", ws.preferredModelUpdates[1].model.Model)
}

func TestToggleThinkingPersistsWorkspaceScope(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Provider: "test-provider",
				Model:    "large-model",
			},
		},
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents: map[string]config.Agent{
			config.AgentCoder: {Model: config.SelectedModelTypeLarge},
		},
	}
	ws := &testWorkspace{cfg: cfg}
	ui := newTestUIWithConfig(t, cfg)
	ui.com.Workspace = ws

	ui.toggleThinking()

	require.Len(t, ws.preferredModelUpdates, 1)
	require.Equal(t, config.ScopeWorkspace, ws.preferredModelUpdates[0].scope)
	require.Equal(t, config.SelectedModelTypeLarge, ws.preferredModelUpdates[0].modelType)
	require.True(t, ws.preferredModelUpdates[0].model.Think)
}

func TestSelectReasoningEffortPersistsWorkspaceScope(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Provider: "test-provider",
				Model:    "large-model",
			},
		},
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents: map[string]config.Agent{
			config.AgentCoder: {Model: config.SelectedModelTypeLarge},
		},
	}
	ws := &testWorkspace{cfg: cfg}
	ui := newTestUIWithConfig(t, cfg)
	ui.com.Workspace = ws

	ui.selectReasoningEffort("high")

	require.Len(t, ws.preferredModelUpdates, 1)
	require.Equal(t, config.ScopeWorkspace, ws.preferredModelUpdates[0].scope)
	require.Equal(t, config.SelectedModelTypeLarge, ws.preferredModelUpdates[0].modelType)
	require.Equal(t, "high", ws.preferredModelUpdates[0].model.ReasoningEffort)
}

func newTestUIWithConfig(t *testing.T, cfg *config.Config) *UI {
	t.Helper()

	return &UI{
		com: &common.Common{
			Workspace: &testWorkspace{cfg: cfg},
		},
	}
}

// testWorkspace is a minimal [workspace.Workspace] stub for unit tests.
type testWorkspace struct {
	workspace.Workspace
	cfg                   *config.Config
	defaultSmallModel     config.SelectedModel
	preferredModelUpdates []preferredModelUpdate
}

type preferredModelUpdate struct {
	scope     config.Scope
	modelType config.SelectedModelType
	model     config.SelectedModel
}

func (w *testWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *testWorkspace) PermissionSkipRequests() bool {
	return false
}

func (w *testWorkspace) ProjectNeedsInitialization() (bool, error) {
	return false, nil
}

func (w *testWorkspace) AgentIsReady() bool {
	return false
}

func (w *testWorkspace) AgentIsBusy() bool {
	return false
}

func (w *testWorkspace) UpdatePreferredModel(scope config.Scope, modelType config.SelectedModelType, model config.SelectedModel) error {
	w.preferredModelUpdates = append(w.preferredModelUpdates, preferredModelUpdate{
		scope:     scope,
		modelType: modelType,
		model:     model,
	})
	return nil
}

func (w *testWorkspace) GetDefaultSmallModel(string) config.SelectedModel {
	return w.defaultSmallModel
}

func (w *testWorkspace) UpdateAgentModel(context.Context) error {
	return nil
}

func ptr[T any](v T) *T {
	return &v
}
