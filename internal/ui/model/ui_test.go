package model

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
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
	cfg               *config.Config
	setMainCalledWith string
	updateCalls       int
}

func (w *testWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *testWorkspace) AgentSetMain(agentID string) error {
	w.setMainCalledWith = agentID
	return nil
}

func (w *testWorkspace) UpdateAgentModel(context.Context) error {
	w.updateCalls++
	return nil
}

func (w *testWorkspace) PermissionSkipRequests() bool {
	return false
}

func TestDefaultKeyMapHasShiftTab(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	require.Equal(t, []string{"shift+tab"}, km.ShiftTab.Keys())
}

func TestToggleInputMode(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	ws := &testWorkspace{cfg: cfg}
	ui := &UI{
		com: &common.Common{
			Workspace: ws,
		},
		mode:     uiInputModeCode,
		textarea: textarea.New(),
	}

	msg := ui.toggleInputMode()()
	require.NotNil(t, msg)
	require.Equal(t, uiInputModePlan, ui.mode)
	require.Equal(t, config.AgentPlan, ws.setMainCalledWith)
	require.Equal(t, 1, ws.updateCalls)

	msg = ui.toggleInputMode()()
	require.NotNil(t, msg)
	require.Equal(t, uiInputModeCode, ui.mode)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
	require.Equal(t, 2, ws.updateCalls)
}

func newPlanUI(t *testing.T, sessionID string) (*UI, *testWorkspace) {
	t.Helper()
	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	ws := &testWorkspace{cfg: cfg}
	var sess *session.Session
	if sessionID != "" {
		s := session.Session{ID: sessionID}
		sess = &s
	}
	u := &UI{
		com: &common.Common{
			Workspace: ws,
		},
		mode:     uiInputModePlan,
		textarea: textarea.New(),
		dialog:   dialog.NewOverlay(),
		session:  sess,
	}
	return u, ws
}

func isPlanHandoffInline(u *UI) bool {
	_, ok := u.activeInline.(*dialog.PlanHandoffInline)
	return ok
}

func TestHandlePlanHandoff_MarkerOpensInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "Here is the plan.\n<!-- CRUSH_PLAN_READY -->",
	})
	require.True(t, isPlanHandoffInline(u))
}

func TestHandlePlanHandoff_NoMarkerNoInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "Here is the plan without marker.",
	})
	require.Nil(t, u.activeInline)
}

func TestHandlePlanHandoff_MarkerInProseNoInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	// The marker mentioned mid-sentence must not trigger a handoff; it
	// only counts when emitted on a line by itself.
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "I will end with <!-- CRUSH_PLAN_READY --> once the plan is done.",
	})
	require.Nil(t, u.activeInline)
}

func TestHandlePlanHandoff_MarkerOwnLineWithTrailingText(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	// Marker on its own line still triggers even with trailing notes.
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "Here is the plan.\n<!-- CRUSH_PLAN_READY -->\nLet me know if anything is off.",
	})
	require.True(t, isPlanHandoffInline(u))
}

func TestHandlePlanHandoff_ErrorRunNoInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
		Error:     "something went wrong",
	})
	require.Nil(t, u.activeInline)
}

func TestHandlePlanHandoff_CancelledRunNoInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
		Cancelled: true,
	})
	require.Nil(t, u.activeInline)
}

func TestHandlePlanHandoff_SessionMismatchNoInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-OTHER",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
	})
	require.Nil(t, u.activeInline)
}

func TestHandlePlanHandoff_CodeModeNoInline(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.mode = uiInputModeCode
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
	})
	require.Nil(t, u.activeInline)
}

func TestHandlePlanHandoff_DuplicateGuard(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	rc := notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
	}
	u.handlePlanHandoff(rc)
	require.True(t, isPlanHandoffInline(u))
	first := u.activeInline
	u.handlePlanHandoff(rc) // guard: must not replace the existing inline
	require.Same(t, first, u.activeInline)
}

func TestSetInputMode_SwitchesToCode(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	ws := &testWorkspace{cfg: cfg}
	u := &UI{
		com:      &common.Common{Workspace: ws},
		mode:     uiInputModePlan,
		textarea: textarea.New(),
	}
	u.setInputMode(uiInputModeCode)()
	require.Equal(t, uiInputModeCode, u.mode)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
}

func TestSetInputMode_SwitchesToPlan(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	ws := &testWorkspace{cfg: cfg}
	u := &UI{
		com:      &common.Common{Workspace: ws},
		mode:     uiInputModeCode,
		textarea: textarea.New(),
	}
	u.setInputMode(uiInputModePlan)()
	require.Equal(t, uiInputModePlan, u.mode)
	require.Equal(t, config.AgentPlan, ws.setMainCalledWith)
}
