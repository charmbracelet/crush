package model

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/joestump-agent/a2tea/event"
	"github.com/stretchr/testify/require"
)

// a2uiWorkspace stubs the workspace calls the A2UI submission path touches.
// The embedded interface panics on anything else, keeping the stub honest.
type a2uiWorkspace struct {
	workspace.Workspace
	prompts []string
}

func (w *a2uiWorkspace) AgentIsReady() bool     { return true }
func (w *a2uiWorkspace) Config() *config.Config { return nil }

func (w *a2uiWorkspace) AgentRun(_ context.Context, _, prompt string, _ ...message.Attachment) error {
	w.prompts = append(w.prompts, prompt)
	return nil
}

// a2uiSubmitForm carries a submit and a cancel button plus a pre-filled
// TextField, mirroring the form shape the a2ui skill teaches the model.
const a2uiSubmitForm = `<a2ui-json>{"version":"v0.9","updateComponents":{"surfaceId":"form","components":[` +
	`{"component":"Card","id":"root","child":"col"},` +
	`{"component":"Column","id":"col","children":["name","btn-send","btn-cancel"]},` +
	`{"component":"TextField","id":"name","label":"Name","value":"Joe"},` +
	`{"component":"Button","id":"btn-send","child":"btn-send-t"},` +
	`{"component":"Text","id":"btn-send-t","text":"Send"},` +
	`{"component":"Button","id":"btn-cancel","child":"btn-cancel-t"},` +
	`{"component":"Text","id":"btn-cancel-t","text":"Cancel"}` +
	`]}}</a2ui-json>`

// newA2UISubmitUI builds a UI with an active session and a chat list holding
// one assistant message with a live A2UI form surface.
func newA2UISubmitUI(t *testing.T, ws *a2uiWorkspace) *UI {
	t.Helper()
	com := common.DefaultCommon(ws)
	m := &UI{
		com:      com,
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
		state:    uiChat,
		focus:    uiFocusMain,
		width:    140,
		height:   45,
		session:  &session.Session{ID: "sess-1"},
	}
	msg := &message.Message{
		ID:   "a2ui-msg",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Fill this in:\n\n" + a2uiSubmitForm},
		},
	}
	item, ok := chat.NewAssistantMessageItem(com.Styles, msg).(*chat.AssistantMessageItem)
	require.True(t, ok)
	m.chat.AppendMessages(item)
	// Render once so the item builds its live surface models.
	_ = item.RawRender(80)
	return m
}

// runCmdTree executes a cmd (and any tea.BatchMsg it fans out into),
// collecting the produced messages.
func runCmdTree(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, runCmdTree(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func TestHandleA2UIButtonClickedStartsAgentTurn(t *testing.T) {
	t.Parallel()

	ws := &a2uiWorkspace{}
	m := newA2UISubmitUI(t, ws)

	cmd := m.handleA2UIButtonClicked(event.ButtonClicked{
		Source: event.Source{ComponentID: "btn-send", SurfaceID: "form"},
		ID:     "btn-send",
	})
	require.NotNil(t, cmd, "a submit button must start a turn")
	runCmdTree(cmd)

	require.Len(t, ws.prompts, 1, "exactly one agent turn must start")
	prompt := ws.prompts[0]
	require.Contains(t, prompt, `"btn-send"`, "the prompt must name the pressed button")
	require.Contains(t, prompt, `"form"`, "the prompt must name the surface")
	require.True(t, strings.Contains(prompt, `name: "Joe"`),
		"the prompt must carry the collected field values, got: %q", prompt)

	// The surface is retired: a second click of the same surface finds no
	// live surface, and the submission goes out with the button identity
	// only (never blocked, never re-reading stale values).
	_, ok := m.chat.RetireA2UISurface("form")
	require.False(t, ok, "the surface must be retired after submission")
}

func TestHandleA2UIButtonClickedCancelDismissesWithoutTurn(t *testing.T) {
	t.Parallel()

	ws := &a2uiWorkspace{}
	m := newA2UISubmitUI(t, ws)

	cmd := m.handleA2UIButtonClicked(event.ButtonClicked{
		Source: event.Source{ComponentID: "btn-cancel", SurfaceID: "form"},
		ID:     "btn-cancel",
	})
	require.Nil(t, cmd, "a cancel button must not start a turn")
	require.Empty(t, ws.prompts)

	// Cancel still retires the surface so it cannot be re-submitted.
	_, ok := m.chat.RetireA2UISurface("form")
	require.False(t, ok, "cancel must retire the surface")
}
