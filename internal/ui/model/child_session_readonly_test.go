package model

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// Finding F6: subagent (child) sessions are read-only. A session with a
// non-empty ParentSessionID must not accept new messages, and loading one
// must not clobber the user's preferred model with whatever the subagent
// happened to run.

// TestSendMessageInChildSessionIsReadOnly pins the read-only guard for child
// sessions: sending a message must not start an agent run and must report
// the read-only status back to the user instead.
func TestSendMessageInChildSessionIsReadOnly(t *testing.T) {
	t.Parallel()

	ws := &testWorkspace{agentReady: true}
	ui := &UI{
		com:     &common.Common{Workspace: ws},
		session: &session.Session{ID: "child-1", ParentSessionID: "parent-1"},
	}

	cmd := ui.sendMessage("hello")
	require.NotNil(t, cmd, "sending a message in a child session must still return a command")

	var gotInfo *util.InfoMsg
	for _, msg := range flattenTeaMsgs(cmd) {
		if info, ok := msg.(util.InfoMsg); ok {
			gotInfo = &info
		}
	}
	require.NotNil(t, gotInfo, "sending a message in a child session must report its read-only status")
	require.Contains(t, gotInfo.Msg, "read-only")
	require.Empty(t, ws.runPrompts, "sending a message in a child session must not start an agent run")
}

// TestEnterInChildSessionKeepsInput verifies that enter in a child session
// is refused before the editor is cleared, so the typed prompt survives, and
// that bang mode can't bypass the guard to run a shell command there.
func TestEnterInChildSessionKeepsInput(t *testing.T) {
	t.Parallel()

	for _, bang := range []bool{false, true} {
		t.Run(fmt.Sprintf("bang=%t", bang), func(t *testing.T) {
			t.Parallel()

			ws := &testWorkspace{cfg: sessionRestoreConfig(), agentReady: true}
			ui := newRestoreModelUI(ws, &session.Session{ID: "child-1", ParentSessionID: "parent-1"})
			ui.bangMode = bang
			ui.textarea.SetValue("hello")

			cmd := ui.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

			require.Equal(t, "hello", ui.textarea.Value(), "a refused send must not discard the typed prompt")
			require.Empty(t, ws.runPrompts)
			var infos []string
			for _, msg := range flattenTeaMsgs(cmd) {
				if info, ok := msg.(util.InfoMsg); ok {
					infos = append(infos, info.Msg)
				}
			}
			require.Len(t, infos, 1)
			require.Contains(t, infos[0], "read-only")
		})
	}
}

// restoreModelWorkspace is a workspace.Workspace stub for exercising
// restoreModelFromSession through a real loadSessionMsg. It embeds the
// interface so any method the load path doesn't otherwise need panics
// loudly instead of silently returning zero values.
type restoreModelWorkspace struct {
	workspace.Workspace
	cfg            *config.Config
	preferredCalls []config.SelectedModel
}

func (w *restoreModelWorkspace) Config() *config.Config { return w.cfg }

func (w *restoreModelWorkspace) WorkingDir() string { return "" }

func (w *restoreModelWorkspace) RunningSubagents(string) []workspace.RunningSubagentInfo {
	return nil
}

func (w *restoreModelWorkspace) UpdatePreferredModel(_ config.Scope, _ config.SelectedModelType, model config.SelectedModel) error {
	w.preferredCalls = append(w.preferredCalls, model)
	return nil
}

// sessionRestoreConfig builds a config where the last assistant message's
// provider/model differs from the configured large model, and is available,
// so restoreModelFromSession would switch to it if invoked.
func sessionRestoreConfig() *config.Config {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("session-provider", config.ProviderConfig{
		ID:     "session-provider",
		Models: []catwalk.Model{{ID: "session-model"}},
	})
	return &config.Config{
		Providers: providers,
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "current-provider", Model: "current-model"},
			config.SelectedModelTypeSmall: {Provider: "current-provider", Model: "current-small-model"},
		},
	}
}

// newRestoreModelUI builds a UI wired to the given workspace and session,
// with enough state for Update(loadSessionMsg{...}) to run end to end.
func newRestoreModelUI(ws workspace.Workspace, sess *session.Session) *UI {
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
		session:     sess,
		keyMap:      DefaultKeyMap(),
		dialog:      dialog.NewOverlay(),
		attachments: attachments.New(nil, attachments.Keymap{}),
		// A provider that isn't "hyper" resolves to the "default" theme key,
		// matching this, so loading the session never has to swap themes.
		themeKey: "default",
	}
}

// TestLoadChildSessionDoesNotRestoreModel pins the read-only guard for child
// sessions: loading one must not persist the subagent's model as the user's
// preferred model.
func TestLoadChildSessionDoesNotRestoreModel(t *testing.T) {
	t.Parallel()

	ws := &restoreModelWorkspace{cfg: sessionRestoreConfig()}
	ui := newRestoreModelUI(ws, nil)

	msgs := []message.Message{
		{ID: "a1", Role: message.Assistant, Provider: "session-provider", Model: "session-model"},
	}
	ui.Update(loadSessionMsg{
		session:  &session.Session{ID: "child-1", ParentSessionID: "parent-1"},
		messages: msgs,
	})

	require.Empty(t, ws.preferredCalls, "loading a child session must not restore its model as the preferred model")
}

// TestLoadTopLevelSessionRestoresModel is the positive control for
// [TestLoadChildSessionDoesNotRestoreModel]: a top-level session (no
// parent) with the same assistant history must still restore the model,
// so the child-session behavior is a deliberate carve-out, not a broken
// restore path.
func TestLoadTopLevelSessionRestoresModel(t *testing.T) {
	t.Parallel()

	ws := &restoreModelWorkspace{cfg: sessionRestoreConfig()}
	ui := newRestoreModelUI(ws, nil)

	msgs := []message.Message{
		{ID: "a1", Role: message.Assistant, Provider: "session-provider", Model: "session-model"},
	}
	ui.Update(loadSessionMsg{
		session:  &session.Session{ID: "top-1"},
		messages: msgs,
	})

	require.Equal(t, []config.SelectedModel{{Provider: "session-provider", Model: "session-model"}}, ws.preferredCalls,
		"loading a top-level session must still restore its model as the preferred model")
}

// flattenTeaMsgs runs cmd (and, recursively, any tea.BatchMsg it yields) and
// collects every resulting message, in execution order.
func flattenTeaMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, flattenTeaMsgs(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}
