package model

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type historyWorkspace struct {
	workspace.Workspace
}

func (historyWorkspace) Config() *config.Config {
	return &config.Config{}
}

func (historyWorkspace) PermissionMode() permission.PermissionMode {
	return permission.PermissionModeNormal
}

func TestHistoryBangCommandStripsPrefixWhileAlreadyInBangMode(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = historyWorkspace{}
	u.promptHistory.messages = []string{"!echo one", "!echo two"}
	u.promptHistory.index = -1

	require.True(t, u.historyPrev())
	require.True(t, u.bangMode)
	require.Equal(t, "echo one", u.textarea.Value())

	require.True(t, u.historyPrev())
	require.True(t, u.bangMode)
	require.Equal(t, "echo two", u.textarea.Value())
}

type promptHistoryWorkspace struct {
	*countingWorkspace
	messages          []message.Message
	requestedSessions []string
}

func (w *promptHistoryWorkspace) ListUserMessages(_ context.Context, sessionID string) ([]message.Message, error) {
	w.requestedSessions = append(w.requestedSessions, sessionID)
	return w.messages, nil
}

func TestPromptHistoryIncludesPersistedUserMessage(t *testing.T) {
	t.Parallel()

	ws := &promptHistoryWorkspace{countingWorkspace: &countingWorkspace{ready: true}}
	u := newBusyUI(ws.countingWorkspace)
	u.com.Workspace = ws
	warmCaches(u, false)
	u.Update(u.loadPromptHistory()())

	// The initial query finishes before the agent persists the prompt.
	prompt := message.Message{
		ID: "user-1", SessionID: "s1", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "cancel this run"}},
	}
	ws.messages = []message.Message{prompt}
	_, cmd := u.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: prompt})
	for _, msg := range runCmd(cmd) {
		if _, ok := msg.(promptHistoryLoadedMsg); ok {
			u.Update(msg)
		}
	}

	// No next submission or successful completion is needed to recall it.
	require.True(t, u.historyPrev())
	require.Equal(t, "cancel this run", u.textarea.Value())
}

func TestPromptHistoryKeepsNewestSnapshot(t *testing.T) {
	t.Parallel()

	ws := &promptHistoryWorkspace{countingWorkspace: &countingWorkspace{ready: true}}
	u := newBusyUI(ws.countingWorkspace)
	u.com.Workspace = ws
	warmCaches(u, false)
	// The database read finishes early, but its result reaches Update late.
	stale := u.loadPromptHistory()()
	prompt := message.Message{
		ID: "user-1", SessionID: "s1", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "cancel this run"}},
	}
	ws.messages = []message.Message{prompt}
	_, cmd := u.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: prompt})
	for _, msg := range runCmd(cmd) {
		if _, ok := msg.(promptHistoryLoadedMsg); ok {
			u.Update(msg)
		}
	}
	u.Update(stale)

	require.True(t, u.historyPrev())
	require.Equal(t, "cancel this run", u.textarea.Value())
}

func TestPromptHistoryLoadStaysWithOriginalSession(t *testing.T) {
	t.Parallel()

	ws := &promptHistoryWorkspace{countingWorkspace: &countingWorkspace{ready: true}}
	u := newBusyUI(ws.countingWorkspace)
	u.com.Workspace = ws
	warmCaches(u, false)
	u.historyReset()
	cmd := u.loadPromptHistory()
	u.session = &session.Session{ID: "s2"}
	u.promptHistory.messages = []string{"another session's prompt"}
	u.Update(cmd())

	require.Equal(t, []string{"s1"}, ws.requestedSessions)
	require.True(t, u.historyPrev())
	require.Equal(t, "another session's prompt", u.textarea.Value())
}
