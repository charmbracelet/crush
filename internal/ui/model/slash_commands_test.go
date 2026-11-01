package model

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

type slashWorkspace struct {
	workspace.Workspace
	cfg       *config.Config
	reloads   int
	prompts   []string
	askAnswer string
}

func (w *slashWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *slashWorkspace) WorkingDir() string {
	return "/tmp"
}

func (w *slashWorkspace) AgentReadyErr() error {
	return nil
}

func (w *slashWorkspace) AgentRun(_ context.Context, _ string, prompt string, _ ...message.Attachment) error {
	w.prompts = append(w.prompts, prompt)
	return nil
}

func (w *slashWorkspace) AgentAsk(context.Context, string, string) (string, error) {
	return w.askAnswer, nil
}

func (w *slashWorkspace) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *slashWorkspace) ReloadConfig(context.Context) error {
	w.reloads++
	return nil
}

func newSlashUI(ws *slashWorkspace) *UI {
	com := common.DefaultCommon(ws)
	return &UI{
		com:         com,
		status:      NewStatus(com, nil),
		chat:        NewChat(com, config.ScrollbarDefault),
		textarea:    textarea.New(),
		state:       uiChat,
		focus:       uiFocusEditor,
		width:       140,
		height:      45,
		session:     &session.Session{ID: "s1"},
		keyMap:      DefaultKeyMap(),
		dialog:      dialog.NewOverlay(),
		attachments: attachments.New(nil, attachments.Keymap{}),
	}
}

func submitEditor(t *testing.T, m *UI, value string) {
	t.Helper()
	m.textarea.SetValue(value)
	runCmd(m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
}

// runCmd executes a command, draining nested batches so the commands the
// editor returns are actually run.
func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmd(c)
		}
	}
}

func TestEditorSlashReloadReloadsConfig(t *testing.T) {
	t.Parallel()

	ws := &slashWorkspace{cfg: &config.Config{Options: &config.Options{}}}
	m := newSlashUI(ws)

	submitEditor(t, m, "/reload")

	require.Equal(t, 1, ws.reloads)
	require.Empty(t, ws.prompts, "/reload must not be sent to the agent")
}

func TestEditorSlashReloadRequiresExactMatch(t *testing.T) {
	t.Parallel()

	ws := &slashWorkspace{cfg: &config.Config{Options: &config.Options{}}}
	m := newSlashUI(ws)

	submitEditor(t, m, "/reload the config")

	require.Zero(t, ws.reloads)
	require.Equal(t, []string{"/reload the config"}, ws.prompts)
}

type reloadActionDialog struct{}

func (d *reloadActionDialog) ID() string {
	return "reload-test"
}

func (d *reloadActionDialog) HandleMsg(tea.Msg) dialog.Action {
	return dialog.ActionReloadConfig{}
}

func (d *reloadActionDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor {
	return nil
}

func TestReloadConfigCommandReloads(t *testing.T) {
	t.Parallel()

	ws := &slashWorkspace{cfg: &config.Config{Options: &config.Options{}}}
	m := newSlashUI(ws)
	m.dialog = dialog.NewOverlay(&reloadActionDialog{})

	runCmd(m.handleDialogMsg(tea.KeyPressMsg{}))

	require.Equal(t, 1, ws.reloads)
}
