package model

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// restoreTestWorkspace captures UpdatePreferredModel and UpdateAgentModel
// invocations from restoreSessionModels.
type restoreTestWorkspace struct {
	workspace.Workspace
	cfg                *config.Config
	updatePreferred    []preferredCall
	updatePreferredErr error
	updatedAgent       int32
	updateAgentErr     error
}

type preferredCall struct {
	scope     config.Scope
	modelType config.SelectedModelType
	model     config.SelectedModel
}

func (w *restoreTestWorkspace) Config() *config.Config { return w.cfg }

func (w *restoreTestWorkspace) UpdatePreferredModel(scope config.Scope, modelType config.SelectedModelType, model config.SelectedModel) error {
	w.updatePreferred = append(w.updatePreferred, preferredCall{
		scope:     scope,
		modelType: modelType,
		model:     model,
	})
	return w.updatePreferredErr
}

func (w *restoreTestWorkspace) UpdateAgentModel(_ context.Context) error {
	atomic.AddInt32(&w.updatedAgent, 1)
	return w.updateAgentErr
}

func newRestoreUI(t *testing.T, cfg *config.Config) (*UI, *restoreTestWorkspace, *bytes.Buffer) {
	t.Helper()
	ws := &restoreTestWorkspace{cfg: cfg}
	ui := &UI{com: &common.Common{Workspace: ws}}

	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return ui, ws, buf
}

func cfgWithModel(provider, model string) *config.Config {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set(provider, config.ProviderConfig{
		ID: provider,
		Models: []catwalk.Model{
			{ID: model},
		},
	})
	return &config.Config{Providers: providers}
}

func TestRestoreSessionModels_NoRowsIsNoOp(t *testing.T) {
	ui, ws, _ := newRestoreUI(t, cfgWithModel("p", "m"))

	require.Nil(t, ui.restoreSessionModels("sess", nil))
	require.Nil(t, ui.restoreSessionModels("sess", []session.SessionModel{}))
	require.Empty(t, ws.updatePreferred)
	require.Equal(t, int32(0), atomic.LoadInt32(&ws.updatedAgent))
}

func TestRestoreSessionModels_AllValidAppliesAndRefreshesOnce(t *testing.T) {
	cfg := cfgWithModel("p", "large")
	cfg.Providers.Set("p2", config.ProviderConfig{
		ID:     "p2",
		Models: []catwalk.Model{{ID: "small"}},
	})

	ui, ws, _ := newRestoreUI(t, cfg)

	rows := []session.SessionModel{
		{
			SessionID: "sess",
			ModelType: config.SelectedModelTypeLarge,
			Provider:  "p",
			Model:     "large",
			SelectedModel: config.SelectedModel{
				Provider:        "p",
				Model:           "large",
				ReasoningEffort: "high",
			},
		},
		{
			SessionID: "sess",
			ModelType: config.SelectedModelTypeSmall,
			Provider:  "p2",
			Model:     "small",
			SelectedModel: config.SelectedModel{
				Provider: "p2",
				Model:    "small",
			},
		},
	}

	cmd := ui.restoreSessionModels("sess", rows)
	require.NotNil(t, cmd)
	require.Len(t, ws.updatePreferred, 2)

	for _, c := range ws.updatePreferred {
		require.Equal(t, config.ScopeWorkspace, c.scope)
	}

	// Verify the full SelectedModel (including reasoning) was passed through.
	var sawLarge bool
	for _, c := range ws.updatePreferred {
		if c.modelType == config.SelectedModelTypeLarge {
			sawLarge = true
			require.Equal(t, "high", c.model.ReasoningEffort)
		}
	}
	require.True(t, sawLarge)

	// UpdateAgentModel should not run until the cmd executes.
	require.Equal(t, int32(0), atomic.LoadInt32(&ws.updatedAgent))
	_ = cmd()
	require.Equal(t, int32(1), atomic.LoadInt32(&ws.updatedAgent))
}

func TestRestoreSessionModels_UnavailableModelKeepsCurrentAndWarns(t *testing.T) {
	// Config only knows about "p/large"; the small row's provider/model
	// is unavailable.
	ui, ws, buf := newRestoreUI(t, cfgWithModel("p", "large"))

	rows := []session.SessionModel{
		{
			SessionID:     "sess-X",
			ModelType:     config.SelectedModelTypeLarge,
			Provider:      "p",
			Model:         "large",
			SelectedModel: config.SelectedModel{Provider: "p", Model: "large"},
		},
		{
			SessionID:     "sess-X",
			ModelType:     config.SelectedModelTypeSmall,
			Provider:      "missing-provider",
			Model:         "missing-model",
			SelectedModel: config.SelectedModel{Provider: "missing-provider", Model: "missing-model"},
		},
	}

	cmd := ui.restoreSessionModels("sess-X", rows)
	require.NotNil(t, cmd, "valid row should still trigger refresh")
	require.Len(t, ws.updatePreferred, 1)
	require.Equal(t, config.SelectedModelTypeLarge, ws.updatePreferred[0].modelType)

	_ = cmd()
	require.Equal(t, int32(1), atomic.LoadInt32(&ws.updatedAgent))

	logOut := buf.String()
	require.Contains(t, logOut, "session_id=sess-X")
	require.Contains(t, logOut, "model_type=small")
	require.Contains(t, logOut, "provider_id=missing-provider")
	require.Contains(t, logOut, "model_id=missing-model")
	require.Contains(t, strings.ToLower(logOut), "unavailable")
}

func TestRestoreSessionModels_UnknownTypeIsSkipped(t *testing.T) {
	ui, ws, buf := newRestoreUI(t, cfgWithModel("p", "m"))

	rows := []session.SessionModel{
		{
			SessionID:     "sess-Y",
			ModelType:     config.SelectedModelType("medium"),
			Provider:      "p",
			Model:         "m",
			SelectedModel: config.SelectedModel{Provider: "p", Model: "m"},
		},
	}

	cmd := ui.restoreSessionModels("sess-Y", rows)
	require.Nil(t, cmd, "no valid rows means no agent refresh")
	require.Empty(t, ws.updatePreferred)
	require.Equal(t, int32(0), atomic.LoadInt32(&ws.updatedAgent))

	logOut := buf.String()
	require.Contains(t, logOut, "session_id=sess-Y")
	require.Contains(t, logOut, "model_type=medium")
	require.Contains(t, logOut, "provider_id=p")
	require.Contains(t, logOut, "model_id=m")
}

func TestRestoreSessionModels_AllInvalidLeavesEverythingUntouched(t *testing.T) {
	ui, ws, buf := newRestoreUI(t, cfgWithModel("p", "m"))

	rows := []session.SessionModel{
		{
			SessionID:     "sess-Z",
			ModelType:     config.SelectedModelTypeLarge,
			Provider:      "ghost",
			Model:         "ghost",
			SelectedModel: config.SelectedModel{Provider: "ghost", Model: "ghost"},
		},
		{
			SessionID:     "sess-Z",
			ModelType:     config.SelectedModelTypeSmall,
			Provider:      "ghost2",
			Model:         "ghost2",
			SelectedModel: config.SelectedModel{Provider: "ghost2", Model: "ghost2"},
		},
	}

	cmd := ui.restoreSessionModels("sess-Z", rows)
	require.Nil(t, cmd)
	require.Empty(t, ws.updatePreferred)
	require.Equal(t, int32(0), atomic.LoadInt32(&ws.updatedAgent))

	logOut := buf.String()
	require.Contains(t, logOut, "provider_id=ghost")
	require.Contains(t, logOut, "provider_id=ghost2")
}
