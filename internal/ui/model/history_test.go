package model

import (
	"context"
	"sync"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

const sessionID string = "s1"

func TestPromptHistoryNavigationOnSubmission(t *testing.T) {
	t.Parallel()

	ws := &historyTestWorkspace{busy: true}
	m := &UI{com: &common.Common{Workspace: ws}}
	m.session = &session.Session{ID: sessionID}
	m.textarea = textarea.New()
	m.promptHistory.index = -1

	const prompt = "hey there"

	// Phase 1: on submit the history is reloaded concurrently with AgentRun and
	// wins the race, reading the DB before the message is persisted (empty).
	applyHistoryReload(t, m, m.loadPromptHistory())

	// Phase 2: AgentRun persists the user message, then the CreatedEvent reload
	// (reloadHistoryForMessage) picks it up.
	require.NoError(t, ws.AgentRun(context.Background(), m.session.ID, prompt))
	created := message.Message{
		Role:      message.User,
		SessionID: m.session.ID,
		Parts:     []message.ContentPart{message.TextContent{Text: prompt}},
	}
	applyHistoryReload(t, m, m.reloadHistoryForMessage(created))

	// Phase 3: pressing "up" must surface the just-sent prompt.
	require.True(t, m.historyPrev(), "`up` must insert just sent prompt")
	require.Equal(t, prompt, m.textarea.Value())
}

// applyHistoryReload runs a prompt-history command and applies its result the
// same way the model does when handling promptHistoryLoadedMsg in Update.
func applyHistoryReload(t *testing.T, m *UI, cmd tea.Cmd) {
	t.Helper()

	loaded, ok := cmd().(promptHistoryLoadedMsg)
	require.True(t, ok, "command must return promptHistoryLoadedMsg")
	m.setPromptHistory(loaded.messages)
}

type historyTestWorkspace struct {
	workspace.Workspace

	cfg  *config.Config
	busy bool

	mu   sync.Mutex
	msgs []message.Message
}

func (w *historyTestWorkspace) addUserMessage(sessionID, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	msg := message.Message{
		Role:      message.User,
		SessionID: sessionID,
		Parts:     []message.ContentPart{message.TextContent{Text: text}},
	}

	w.msgs = append([]message.Message{msg}, w.msgs...)
}

func (w *historyTestWorkspace) snapshot() []message.Message {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]message.Message, len(w.msgs))
	copy(out, w.msgs)

	return out
}

func (w *historyTestWorkspace) ListUserMessages(_ context.Context, _ string) ([]message.Message, error) {
	return w.snapshot(), nil
}

func (w *historyTestWorkspace) ListAllUserMessages(_ context.Context) ([]message.Message, error) {
	return w.snapshot(), nil
}

func (w *historyTestWorkspace) AgentIsReady() bool     { return true }
func (w *historyTestWorkspace) AgentIsBusy() bool      { return w.busy }
func (w *historyTestWorkspace) Config() *config.Config { return w.cfg }

func (w *historyTestWorkspace) CreateSession(_ context.Context, _ string) (session.Session, error) {
	return session.Session{ID: sessionID}, nil
}

func (w *historyTestWorkspace) AgentRun(_ context.Context, sessionID, prompt string, _ ...message.Attachment) error {
	w.addUserMessage(sessionID, prompt)
	return nil
}
