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
	u.promptHistory.messages = []promptHistoryEntry{{text: "!echo one"}, {text: "!echo two"}}
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
	u.promptHistory.messages = []promptHistoryEntry{{text: "another session's prompt"}}
	u.Update(cmd())

	require.Equal(t, []string{"s1"}, ws.requestedSessions)
	require.True(t, u.historyPrev())
	require.Equal(t, "another session's prompt", u.textarea.Value())
}

func TestPromptHistoryRefreshPreservesDraft(t *testing.T) {
	t.Parallel()

	for _, change := range []string{"unchanged", "prepended", "removed"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()

			old := message.Message{
				ID: "old", SessionID: "s1", Role: message.User,
				Parts: []message.ContentPart{message.TextContent{Text: "old prompt"}},
			}
			prompt := message.Message{ID: "new", SessionID: "s1", Role: message.User}
			ws := &promptHistoryWorkspace{
				countingWorkspace: &countingWorkspace{ready: true},
				messages:          []message.Message{old},
			}
			u := newBusyUI(ws.countingWorkspace)
			u.com.Workspace = ws
			warmCaches(u, false)
			u.Update(u.loadPromptHistory()())
			u.textarea.SetValue("unsent draft")
			require.True(t, u.historyPrev())

			if change != "unchanged" {
				prompt.Parts = []message.ContentPart{message.TextContent{Text: "new prompt"}}
				ws.messages = []message.Message{prompt}
				if change == "prepended" {
					ws.messages = append(ws.messages, old)
				}
			}
			_, cmd := u.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: prompt})
			for _, msg := range runCmd(cmd) {
				if _, ok := msg.(promptHistoryLoadedMsg); ok {
					u.Update(msg)
				}
			}

			if change != "removed" {
				require.Equal(t, "old prompt", u.textarea.Value())
				require.True(t, u.historyNext())
				if change == "prepended" {
					require.Equal(t, "new prompt", u.textarea.Value())
					require.True(t, u.historyNext())
				}
			}
			require.Equal(t, "unsent draft", u.textarea.Value())
		})
	}
}

func TestPromptHistoryIgnoresHiddenContinuation(t *testing.T) {
	t.Parallel()

	ws := &promptHistoryWorkspace{countingWorkspace: &countingWorkspace{ready: true}}
	old := message.Message{
		ID: "old", SessionID: "s1", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "old prompt"}},
	}
	ws.messages = []message.Message{old}
	u := newBusyUI(ws.countingWorkspace)
	u.com.Workspace = ws
	warmCaches(u, false)
	u.Update(u.loadPromptHistory()())
	hidden := message.Message{
		ID: "hidden", SessionID: "s1", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Implement the plan.", Hidden: true}},
	}
	ws.messages = []message.Message{hidden, old}
	_, cmd := u.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: hidden})
	for _, msg := range runCmd(cmd) {
		if _, ok := msg.(promptHistoryLoadedMsg); ok {
			u.Update(msg)
		}
	}
	require.True(t, u.historyPrev())
	require.Equal(t, "old prompt", u.textarea.Value())
}

func TestPromptHistoryRefreshKeepsDuplicateSelection(t *testing.T) {
	t.Parallel()

	ws := &promptHistoryWorkspace{countingWorkspace: &countingWorkspace{ready: true}}
	for _, item := range []struct{ id, text string }{
		{"first", "repeat prompt"}, {"middle", "middle prompt"}, {"last", "repeat prompt"},
	} {
		ws.messages = append(ws.messages, message.Message{
			ID: item.id, SessionID: "s1", Role: message.User,
			Parts: []message.ContentPart{message.TextContent{Text: item.text}},
		})
	}
	u := newBusyUI(ws.countingWorkspace)
	u.com.Workspace = ws
	warmCaches(u, false)
	u.Update(u.loadPromptHistory()())
	u.textarea.SetValue("unsent draft")
	for range 3 {
		require.True(t, u.historyPrev())
	}
	prompt := message.Message{
		ID: "new", SessionID: "s1", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "new prompt"}},
	}
	ws.messages = append([]message.Message{prompt}, ws.messages...)
	_, cmd := u.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: prompt})
	for _, msg := range runCmd(cmd) {
		if _, ok := msg.(promptHistoryLoadedMsg); ok {
			u.Update(msg)
		}
	}
	require.Equal(t, "repeat prompt", u.textarea.Value())
	require.True(t, u.historyNext())
	require.Equal(t, "middle prompt", u.textarea.Value())
	u.handleHistoryEscape(nil)
	require.Equal(t, "unsent draft", u.textarea.Value())
}

func TestPromptHistoryRefreshKeepsShellCommandSelection(t *testing.T) {
	t.Parallel()

	ws := &promptHistoryWorkspace{
		countingWorkspace: &countingWorkspace{ready: true},
		messages: []message.Message{{
			ID: "shells", SessionID: "s1", Role: message.User,
			Parts: []message.ContentPart{
				message.ShellCommand{Command: "echo same"},
				message.ShellCommand{Command: "echo middle"},
				message.ShellCommand{Command: "echo same"},
			},
		}},
	}
	u := newBusyUI(ws.countingWorkspace)
	u.com.Workspace = ws
	warmCaches(u, false)
	u.Update(u.loadPromptHistory()())
	u.textarea.SetValue("unsent draft")
	for range 3 {
		require.True(t, u.historyPrev())
	}
	prompt := message.Message{
		ID: "new", SessionID: "s1", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "new prompt"}},
	}
	ws.messages = append([]message.Message{prompt}, ws.messages...)
	_, cmd := u.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: prompt})
	for _, msg := range runCmd(cmd) {
		if _, ok := msg.(promptHistoryLoadedMsg); ok {
			u.Update(msg)
		}
	}
	require.True(t, u.bangMode)
	require.Equal(t, "echo same", u.textarea.Value())
	require.True(t, u.historyNext())
	require.Equal(t, "echo middle", u.textarea.Value())
	u.handleHistoryEscape(nil)
	require.False(t, u.bangMode)
	require.Equal(t, "unsent draft", u.textarea.Value())
}
