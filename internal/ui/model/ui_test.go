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
	"github.com/charmbracelet/crush/internal/permission"
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

func TestMouseMode(t *testing.T) {
	t.Parallel()

	t.Run("returns no mouse mode when disabled", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, tea.MouseModeNone, mouseMode(false, false))
		require.Equal(t, tea.MouseModeNone, mouseMode(false, true))
	})

	t.Run("returns cell motion when enabled and no inline editor is active", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, tea.MouseModeCellMotion, mouseMode(true, false))
	})

	t.Run("returns all motion when enabled and an inline editor is active", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, tea.MouseModeAllMotion, mouseMode(true, true))
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
	// level is the permission axis and mainAgent the purpose axis, so a test
	// can assert which one an action moved.
	level        permission.Level
	mainAgent    string
	mainAgents   []string
	runHidden    []bool
	compactCalls []bool
}

func (w *testWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *testWorkspace) WorkingDir() string {
	return "/tmp/crush-test"
}

func (w *testWorkspace) AgentSetMain(agentID string) error {
	w.setMainCalledWith = agentID
	return nil
}

func (w *testWorkspace) UpdateAgentModel(context.Context) error {
	w.updateCalls++
	return nil
}

func (w *testWorkspace) PermissionLevel() permission.Level { return w.level }

func (w *testWorkspace) PermissionSetLevel(level permission.Level) { w.level = level }

func (w *testWorkspace) AgentMainID() string {
	if w.mainAgent == "" {
		return config.AgentCoder
	}
	return w.mainAgent
}

func (w *testWorkspace) AgentMainCandidates() []string {
	if len(w.mainAgents) > 0 {
		return w.mainAgents
	}
	return workspace.DefaultMainAgents
}

func (w *testWorkspace) AgentIsReady() bool {
	return w.agentReady
}

func (w *testWorkspace) AgentIsBusy() bool {
	return w.agentBusy
}

func (w *testWorkspace) AgentReadyErr() error {
	if !w.agentReady {
		return workspace.ErrAgentNotInitialized
	}
	return nil
}

func (w *testWorkspace) AgentRun(ctx context.Context, _ string, prompt string, _ ...message.Attachment) error {
	w.runPrompts = append(w.runPrompts, prompt)
	w.runHidden = append(w.runHidden, message.HiddenUserMessage(ctx))
	return nil
}

func (w *testWorkspace) SetCompactMode(scope config.Scope, compact bool) error {
	w.compactCalls = append(w.compactCalls, compact)
	return nil
}

func TestDefaultKeyMapHasShiftTab(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	require.Equal(t, []string{"shift+tab"}, km.ShiftTab.Keys())
}

// TestCyclePurposeWalksPublishedAgents pins the purpose axis: Shift+Tab moves
// which agent serves the turn and never touches the permission level.
func TestCyclePurposeWalksPublishedAgents(t *testing.T) {
	t.Parallel()
	ui, ws := newPlanUI(t, "sess-1")
	ui.purpose = config.AgentCoder
	ui.levelCache.set(permission.LevelAuto)
	ws.level = permission.LevelAuto

	for _, want := range []string{config.AgentPlan, config.AgentCoder} {
		applyPurposeSwitchMsg(ui, ui.cyclePurpose())
		require.Equal(t, want, ui.purpose)
		require.Equal(t, permission.LevelAuto, ws.level,
			"the purpose cycle must leave the permission axis alone")
	}
}

// TestCyclePurposeFollowsTheWorkspaceList: candidates come from the
// workspace, so a purpose added upstream joins the cycle without any TUI
// change and the loop still returns to where it started.
func TestCyclePurposeFollowsTheWorkspaceList(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	u.purpose = config.AgentCoder
	ws.mainAgents = []string{config.AgentCoder, config.AgentPlan, "sysadmin"}

	var seen []string
	for range len(ws.mainAgents) {
		applyPurposeSwitchMsg(u, u.cyclePurpose())
		seen = append(seen, u.purpose)
	}
	require.Equal(t, []string{config.AgentPlan, "sysadmin", config.AgentCoder}, seen)
}

// TestCyclePermissionLevelWalksLevels pins the permission axis: Ctrl+Y moves
// ask -> auto -> bypass -> ask and never touches the purpose, including while
// planning, where the old single cycle used to route out through code.
func TestCyclePermissionLevelWalksLevels(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	u.purpose = config.AgentPlan

	for _, want := range []permission.Level{permission.LevelAuto, permission.LevelBypass, permission.LevelPrompt} {
		require.Equal(t, want, u.cyclePermissionLevel())
		require.Equal(t, want, ws.level)
	}
	require.Equal(t, config.AgentPlan, u.purpose,
		"the permission cycle must leave the purpose axis alone")
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
	com := &common.Common{
		Workspace: ws,
		Styles:    &sty,
	}
	u := &UI{
		com:      com,
		purpose:  config.AgentPlan,
		textarea: textarea.New(),
		dialog:   dialog.NewOverlay(),
		session:  sess,
		chat:     NewChat(com, config.ScrollbarDefault),
	}
	return u, ws
}

// applyPurposeSwitchMsg runs a setPurpose command to completion the way the
// real event loop does: execute the async switch, then feed the resulting
// modeSwitchedMsg through the finalizer. Returns the finalizer's cmds so
// tests can execute them when they care about the queued work.
func applyPurposeSwitchMsg(u *UI, cmd tea.Cmd) []tea.Cmd {
	t := cmd()
	switched, ok := t.(modeSwitchedMsg)
	if !ok {
		return nil
	}
	u.modeSwitching = false
	return u.applyPurposeSwitch(switched)
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

func TestToggleSidebarKeyBinding(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ws := &testWorkspace{cfg: &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}}
	com := &common.Common{Workspace: ws, Styles: &sty}
	keyMap := DefaultKeyMap()
	att := attachments.New(nil, attachments.Keymap{
		DeleteMode: keyMap.Editor.AttachmentDeleteMode,
		DeleteAll:  keyMap.Editor.DeleteAllAttachments,
		Escape:     keyMap.Editor.Escape,
	})
	u := &UI{
		com:         com,
		keyMap:      keyMap,
		state:       uiChat,
		focus:       uiFocusSidebar,
		session:     &session.Session{ID: "sess-1"},
		chat:        NewChat(com, config.ScrollbarDefault),
		textarea:    textarea.New(),
		dialog:      dialog.NewOverlay(),
		attachments: att,
		width:       140,
		height:      45,
	}
	u.status = NewStatus(com, u)

	// Ctrl+b hides the sidebar and moves focus off it, persisting compact
	// mode.
	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	require.True(t, u.forceCompactMode)
	require.True(t, u.isCompact)
	require.Equal(t, uiFocusEditor, u.focus)
	require.Equal(t, []bool{true}, ws.compactCalls)

	// Ctrl+b again shows the sidebar.
	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	require.False(t, u.forceCompactMode)
	require.False(t, u.isCompact)
	require.Equal(t, []bool{true, false}, ws.compactCalls)

	// The binding is advertised in the help bar while the sidebar is
	// available.
	var shortHelp []string
	for _, b := range u.ShortHelp() {
		shortHelp = append(shortHelp, b.Help().Desc)
	}
	require.Contains(t, shortHelp, "toggle sidebar")

	var fullHelp []string
	for _, row := range u.FullHelp() {
		for _, b := range row {
			fullHelp = append(fullHelp, b.Help().Desc)
		}
	}
	require.Contains(t, fullHelp, "toggle sidebar")
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
	u.purpose = config.AgentCoder
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
	require.Equal(t, config.AgentPlan, u.purpose)

	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok)
	for _, nested := range batch {
		if nested != nil {
			nested()
		}
	}
	require.Equal(t, []string{"Revise the scope"}, ws.runPrompts)
	require.Equal(t, config.AgentPlan, u.purpose)
}

func TestPlanHandoffBlurPreservesAndRestoresInline(t *testing.T) {
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
	require.Equal(t, "focus editor", blurredHelp[0].Help().Desc)
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

	u.Update(tea.MouseClickMsg{X: 4, Y: 12})
	u.Update(tea.MouseMotionMsg{X: 8, Y: 12})
	_, cmd := u.Update(tea.MouseReleaseMsg{X: 8, Y: 12})

	require.Equal(t, "copy", inline.SelectedText())
	require.NotNil(t, cmd)
}

func TestSetPurpose_SwitchesToCode(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	ws := &testWorkspace{cfg: cfg}
	u := &UI{
		com:      &common.Common{Workspace: ws},
		purpose:  config.AgentPlan,
		textarea: textarea.New(),
	}
	applyPurposeSwitchMsg(u, u.setPurpose(config.AgentCoder))
	require.Equal(t, config.AgentCoder, u.purpose)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
}

func TestSetPurpose_SwitchesToPlan(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	ws := &testWorkspace{cfg: cfg}
	u := &UI{
		com:      &common.Common{Workspace: ws},
		purpose:  config.AgentCoder,
		textarea: textarea.New(),
	}
	applyPurposeSwitchMsg(u, u.setPurpose(config.AgentPlan))
	require.Equal(t, config.AgentPlan, u.purpose)
	require.Equal(t, config.AgentPlan, ws.setMainCalledWith)
}

func TestCyclePurpose_BlockedWhileAgentBusy(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	ws.agentReady = true
	ws.agentBusy = true
	// isAgentBusy reads the memoized cache, so seed it with the same value
	// the workspace stub reports.
	u.agentBusyCache.set(true)

	msg := u.cyclePurpose()()
	require.Equal(t, config.AgentPlan, u.purpose, "mode must not change while the agent is busy")
	require.Empty(t, ws.setMainCalledWith)
	info, ok := msg.(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeWarn, info.Type)
}

func TestSetPurpose_TracksModeSwitching(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")

	cmd := u.setPurpose(config.AgentCoder)
	require.True(t, u.modeSwitching, "flag must be set until the async model update completes")

	msg, ok := cmd().(modeSwitchedMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	require.Equal(t, config.AgentCoder, msg.agentID)
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

	cmd := inline.OnConfirm(false)
	require.NotNil(t, cmd)
	// The switch is async: the mode only changes once the backend settles.
	require.Equal(t, config.AgentPlan, u.purpose)
	cmds := applyPurposeSwitchMsg(u, cmd)
	require.Equal(t, config.AgentCoder, u.purpose)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
	require.Empty(t, u.planReadySessionID)

	// The confirmed plan continues with a hidden implement prompt.
	for _, c := range cmds {
		if c == nil {
			continue
		}
		if batch, ok := c().(tea.BatchMsg); ok {
			for _, nested := range batch {
				if nested != nil {
					nested()
				}
			}
		}
	}
	require.Equal(t, []string{"Implement the plan."}, ws.runPrompts)
	require.Equal(t, []bool{true}, ws.runHidden)
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
	applyPurposeSwitchMsg(u, cmd)
	require.Equal(t, config.AgentCoder, u.purpose)
	require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
	require.Empty(t, u.planReadySessionID)
	require.Nil(t, u.activeInline)
}

func TestResetPlanModeState_NoopInCodeMode(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	u.purpose = config.AgentCoder

	cmd := u.resetPlanModeState()
	require.Nil(t, cmd)
	require.Equal(t, config.AgentCoder, u.purpose)
	require.Empty(t, ws.setMainCalledWith)
}

func TestPlanHandoffExplicitPermissionMode(t *testing.T) {
	t.Parallel()
	for _, bypass := range []bool{false, true} {
		u, ws := newPlanUI(t, "sess-1")
		// Start from the opposite level so the assertion shows that the
		// handoff actually sets it rather than leaving whatever was there.
		start := permission.LevelBypass
		if bypass {
			start = permission.LevelPrompt
		}
		ws.level = start
		u.levelCache.set(start)

		u.openPlanHandoff()
		inline := u.activeInline.(*dialog.PlanHandoffInline)
		cmd := inline.OnConfirm(bypass)

		want := permission.LevelPrompt
		if bypass {
			want = permission.LevelBypass
		}
		require.Equal(t, want, ws.level)
		require.Empty(t, ws.runPrompts, "wait for the coder model to finish switching")
		switched := cmd().(modeSwitchedMsg)
		require.NoError(t, switched.err)
		require.Equal(t, "sess-1", switched.continueSessionID)
	}
}

// TestPlanPromptIgnoresBypassLevel: planning has no mutating tools, so its
// prompt stays the plan prompt even at the bypass level.
func TestPlanPromptIgnoresBypassLevel(t *testing.T) {
	t.Parallel()
	u, _ := newPlanUI(t, "sess-1")
	u.textarea.SetWidth(40)
	u.textarea.Focus()
	u.levelCache.set(permission.LevelBypass)
	u.setEditorPrompt()
	require.Contains(t, u.textarea.View(), "⏸")
	require.NotContains(t, u.textarea.View(), " ! ")
}

func TestGeneratedPlanContinuationIsHidden(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	ws.agentReady = true
	for _, hidden := range []bool{true, false} {
		batch := u.sendMessageInternal("Implement the plan.", hidden)().(tea.BatchMsg)
		for _, cmd := range batch {
			if cmd != nil {
				cmd()
			}
		}
	}
	require.Equal(t, []bool{true, false}, ws.runHidden)
	require.Equal(t, []string{"Implement the plan.", "Implement the plan."}, ws.runPrompts)
}

// TestCyclePurposePreservesPermissionLevel: entering planning must not touch
// the permission axis, whatever the user had it at.
func TestCyclePurposePreservesPermissionLevel(t *testing.T) {
	t.Parallel()
	for _, level := range []permission.Level{permission.LevelPrompt, permission.LevelAuto, permission.LevelBypass} {
		u, ws := newPlanUI(t, "sess-1")
		u.purpose = config.AgentCoder
		u.levelCache.set(level)
		ws.level = level

		applyPurposeSwitchMsg(u, u.cyclePurpose())
		require.Equal(t, config.AgentPlan, u.purpose)
		require.Equal(t, level, ws.level, "planning must not change the permission level")
	}
}

// TestCyclePermissionLevelWhilePlanning is what the old plan-to-YOLO shortcut
// stood in for: the permission key now works from every purpose, so planning
// is no longer a special case that has to be escaped to change it.
func TestCyclePermissionLevelWhilePlanning(t *testing.T) {
	t.Parallel()
	u, ws := newPlanUI(t, "sess-1")
	u.purpose = config.AgentPlan
	u.levelCache.set(permission.LevelAuto)

	require.Equal(t, permission.LevelBypass, u.cyclePermissionLevel())
	require.Equal(t, permission.LevelBypass, ws.level)
	require.Equal(t, config.AgentPlan, u.purpose, "the permission cycle must stay in planning")
	require.Empty(t, ws.setMainCalledWith, "and must not switch agents")
}
