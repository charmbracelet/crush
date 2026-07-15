package model

import (
	"context"
	"image"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
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
	agentReady        bool
	agentBusy         bool
	runPrompts        []string
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

func (w *testWorkspace) AgentIsReady() bool {
	return w.agentReady
}

func (w *testWorkspace) AgentIsBusy() bool {
	return w.agentBusy
}

func (w *testWorkspace) AgentRun(_ context.Context, _ string, prompt string, _ ...message.Attachment) error {
	w.runPrompts = append(w.runPrompts, prompt)
	return nil
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
	sty := styles.CharmtonePantera()
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
			Styles:    &sty,
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

func TestHandlePlanHandoff_RequestChangesSendsFeedbackInPlanMode(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	ws.agentReady = true
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "Here is the plan.\n<!-- CRUSH_PLAN_READY -->",
	})

	inline, ok := u.activeInline.(*dialog.PlanHandoffInline)
	require.True(t, ok)
	require.NotNil(t, inline.OnRequestChanges)
	require.NotNil(t, inline.OnConfirm)

	cmd := inline.OnRequestChanges("Revise the scope")
	require.NotNil(t, cmd)
	require.Equal(t, uiInputModePlan, u.mode)

	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok)
	for _, nested := range batch {
		if nested != nil {
			nested()
		}
	}
	require.Equal(t, []string{"Revise the scope"}, ws.runPrompts)
	require.Equal(t, uiInputModePlan, u.mode)
}

func TestPlanHandoffCollapsePreservesAndRestoresInline(t *testing.T) {
	t.Parallel()

	u, _ := newPlanUI(t, "sess-1")
	u.keyMap = DefaultKeyMap()
	u.state = uiChat
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "Here is the plan.\n<!-- CRUSH_PLAN_READY -->",
	})
	inline := u.activeInline

	done, collapseCmd := inline.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, done)
	require.NotNil(t, collapseCmd)
	u.Update(collapseCmd())

	require.Same(t, inline, u.activeInline)
	require.Equal(t, uiFocusMain, u.focus)
	blurredHelp := u.ShortHelp()
	require.Equal(t, "review plan", blurredHelp[0].Help().Desc)
	for _, binding := range blurredHelp {
		require.NotEqual(t, "confirm", binding.Help().Desc)
	}

	u.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Same(t, inline, u.activeInline)
	require.Equal(t, uiFocusEditor, u.focus)
	require.Equal(t, "confirm", u.ShortHelp()[1].Help().Desc)
}

func TestPlanHandoffCollapsedClickRestoresFocus(t *testing.T) {
	t.Parallel()

	u, _ := newPlanUI(t, "sess-1")
	u.state = uiChat
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "Here is the plan.\n<!-- CRUSH_PLAN_READY -->",
	})
	u.focusActiveInline(uiFocusMain)
	u.layout.editor = image.Rect(0, 5, 80, 7)

	u.handleClickFocus(tea.MouseClickMsg{X: 1, Y: 5})

	require.True(t, isPlanHandoffInline(u))
	require.Equal(t, uiFocusEditor, u.focus)
}

func TestPlanHandoffRequestChangesAllowsChatTextSelection(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}}
	u.dialog = dialog.NewOverlay()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.layout.main = image.Rect(0, 0, 60, 10)
	u.chat.SetSize(u.layout.main.Dx(), u.layout.main.Dy())
	u.chat.SetMessages(chat.NewAssistantMessageItem(u.com.Styles, &message.Message{
		ID:   "plan",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Selectable plan text"},
		},
	}))

	inline := dialog.NewPlanHandoffInline(u.com)
	inline.SetFocused(true)
	inline.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	u.activeInline = inline
	u.focus = uiFocusEditor

	startY := -1
	for y := 0; y < u.layout.main.Dy(); y++ {
		if handled, _ := u.chat.HandleMouseDown(4, y); handled {
			startY = y
			break
		}
	}
	require.NotEqual(t, -1, startY, "expected selectable plan content in the chat viewport")

	u.Update(tea.MouseMotionMsg{X: 18, Y: startY})

	require.True(t, u.chat.HasHighlight(),
		"request changes must not suppress chat selection dragging")
}

func TestPlanHandoffRequestChangesRoutesEditorTextSelection(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	inline := dialog.NewPlanHandoffInline(u.com)
	inline.SetFocused(true)
	inline.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	inline.HandlePaste(tea.PasteMsg{Content: "copy me"})
	u.activeInline = inline
	u.layout.editor = image.Rect(0, 10, 80, 10+inline.Height(80))
	scr := uv.NewScreenBuffer(80, u.layout.editor.Max.Y)
	inline.Draw(scr, u.layout.editor)

	u.Update(tea.MouseClickMsg{X: 2, Y: 12})
	u.Update(tea.MouseMotionMsg{X: 6, Y: 12})
	_, cmd := u.Update(tea.MouseReleaseMsg{X: 6, Y: 12})

	require.Equal(t, "copy", inline.SelectedText())
	require.NotNil(t, cmd)
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

func TestToggleInputMode_BlockedWhileAgentBusy(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	ws.agentReady = true
	ws.agentBusy = true

	msg := u.toggleInputMode()()
	require.Equal(t, uiInputModePlan, u.mode, "mode must not change while the agent is busy")
	require.Empty(t, ws.setMainCalledWith)
	info, ok := msg.(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeWarn, info.Type)
}

func TestSetInputMode_TracksModeSwitching(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")

	cmd := u.setInputMode(uiInputModeCode)
	require.True(t, u.modeSwitching, "flag must be set until the async model update completes")

	msg, ok := cmd().(modeSwitchedMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	require.Equal(t, "code", msg.label)
}

func TestHandlePlanHandoff_SetsPendingPlan(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
	})
	require.Equal(t, "sess-1", u.planReadySessionID)
}

func TestHandlePlanHandoff_DismissKeepsPendingAndReopens(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
	})
	require.True(t, isPlanHandoffInline(u))

	// "Keep editing" dismisses the inline prompt but keeps the plan pending.
	u.activeInline = nil
	require.Equal(t, "sess-1", u.planReadySessionID)

	// Enter on an empty editor reopens the prompt via openPlanHandoff.
	u.openPlanHandoff()
	require.True(t, isPlanHandoffInline(u))
}

func TestPlanHandoffConfirm_ClearsPendingAndSwitchesMode(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	ws.agentReady = true
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "sess-1",
		Text:      "plan\n<!-- CRUSH_PLAN_READY -->",
	})
	inline, ok := u.activeInline.(*dialog.PlanHandoffInline)
	require.True(t, ok)

	cmd := inline.OnConfirm()
	require.NotNil(t, cmd)
	require.Equal(t, uiInputModeCode, u.mode)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
	require.Empty(t, u.planReadySessionID)
}

func TestSendMessage_ClearsPendingPlan(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	ws.agentReady = true
	u.setPlanReadyPending("sess-1")

	cmd := u.sendMessage("a new prompt that supersedes the plan")
	require.NotNil(t, cmd)
	require.Empty(t, u.planReadySessionID)
}

func TestResetPlanModeState(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	u.setPlanReadyPending("sess-1")
	u.openPlanHandoff()
	require.True(t, isPlanHandoffInline(u))

	cmd := u.resetPlanModeState()
	require.NotNil(t, cmd)
	require.Equal(t, uiInputModeCode, u.mode)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
	require.Empty(t, u.planReadySessionID)
	require.Nil(t, u.activeInline)
}

func TestResetPlanModeState_NoopInCodeMode(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	u.mode = uiInputModeCode

	cmd := u.resetPlanModeState()
	require.Nil(t, cmd)
	require.Equal(t, uiInputModeCode, u.mode)
	require.Empty(t, ws.setMainCalledWith)
}
