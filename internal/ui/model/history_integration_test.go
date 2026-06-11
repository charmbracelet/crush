package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// extra mock methods required by the full New()/Update path.
func (w *historyTestWorkspace) PermissionSkipRequests() bool              { return false }
func (w *historyTestWorkspace) ProjectNeedsInitialization() (bool, error) { return false, nil }
func (w *historyTestWorkspace) AgentQueuedPrompts(string) int             { return 0 }

// TestCreatedEventReloadsPromptHistory drives the real Update with a
// pubsub.CreatedEvent and asserts the wiring (ui.go CreatedEvent ->
// reloadHistoryForMessage) actually reloads prompt history.
func TestCreatedEventReloadsPromptHistory(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	cfg := &config.Config{
		Options:   &config.Options{TUI: &config.TUIOptions{}},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	ws := &historyTestWorkspace{cfg: cfg}
	com := &common.Common{Workspace: ws, Styles: &st}

	m := New(com, "", false)
	m.session = &session.Session{ID: sessionID}

	// Give the model a size so layout/render don't run at zero dimensions.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(*UI)

	const prompt = "hey there"
	ws.addUserMessage(sessionID, prompt) // message persisted to the "DB"

	created := message.Message{
		Role:      message.User,
		SessionID: sessionID,
		Parts:     []message.ContentPart{message.TextContent{Text: prompt}},
	}

	// Drive the REAL Update with a CreatedEvent and process the resulting cmds.
	model, cmd := m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: created})
	m = model.(*UI)
	for _, msg := range drainCmd(cmd) {
		if _, ok := msg.(promptHistoryLoadedMsg); ok {
			model, _ = m.Update(msg)
			m = model.(*UI)
		}
	}

	require.True(t, m.historyPrev(), "after CreatedEvent up must surface the message")
	require.Equal(t, prompt, m.textarea.Value())
}
