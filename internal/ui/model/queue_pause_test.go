package model

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

type mockQueueCoordinator struct {
	queue          int
	paused         bool
	busy           bool
	cancelSessions []string
}

func (m *mockQueueCoordinator) Run(context.Context, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (m *mockQueueCoordinator) Cancel(sessionID string) {
	m.cancelSessions = append(m.cancelSessions, sessionID)
}
func (m *mockQueueCoordinator) CancelAll()                          {}
func (m *mockQueueCoordinator) IsSessionBusy(string) bool           { return m.busy }
func (m *mockQueueCoordinator) IsBusy() bool                        { return false }
func (m *mockQueueCoordinator) QueuedPrompts(string) int            { return m.queue }
func (m *mockQueueCoordinator) QueuedPromptsList(string) []string   { return nil }
func (m *mockQueueCoordinator) RemoveQueuedPrompt(string, int) bool { return false }
func (m *mockQueueCoordinator) ClearQueue(string)                   {}
func (m *mockQueueCoordinator) PauseQueue(string)                   { m.paused = true }
func (m *mockQueueCoordinator) ResumeQueue(string)                  { m.paused = false }
func (m *mockQueueCoordinator) IsQueuePaused(string) bool           { return m.paused }
func (m *mockQueueCoordinator) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}

func (m *mockQueueCoordinator) GenerateHandoff(context.Context, string, string) (agent.HandoffDraft, error) {
	return agent.HandoffDraft{}, nil
}

func (m *mockQueueCoordinator) ClassifyPermission(context.Context, permission.PermissionRequest) (permission.AutoClassification, error) {
	return permission.AutoClassification{}, nil
}
func (m *mockQueueCoordinator) Model() agent.Model { return agent.Model{} }
func (m *mockQueueCoordinator) EscalationBridge() *permission.EscalationBridge {
	return nil
}

func (m *mockQueueCoordinator) PrepareModelSwitch(context.Context, string, config.SelectedModelType, config.SelectedModel) error {
	return nil
}
func (m *mockQueueCoordinator) UpdateModels(context.Context) error      { return nil }
func (m *mockQueueCoordinator) RefreshTools(context.Context) error      { return nil }
func (m *mockQueueCoordinator) PrioritizeQueuedPrompt(string, int) bool { return false }

func TestSyncPromptQueueTracksPausedState(t *testing.T) {
	t.Parallel()

	coord := &mockQueueCoordinator{queue: 2, paused: true}
	ui := &UI{
		session: &session.Session{ID: "s1"},
		com:     &common.Common{App: &app.App{AgentCoordinator: coord}},
	}

	changed := ui.syncPromptQueue()
	require.True(t, changed)
	require.Equal(t, 2, ui.promptQueue)
	require.True(t, ui.queuePaused)
}

func TestSyncPromptQueueDetectsPauseToggleWithoutQueueSizeChange(t *testing.T) {
	t.Parallel()

	coord := &mockQueueCoordinator{queue: 1, paused: false}
	ui := &UI{
		session: &session.Session{ID: "s1"},
		com:     &common.Common{App: &app.App{AgentCoordinator: coord}},
	}

	changed := ui.syncPromptQueue()
	require.True(t, changed)
	require.False(t, ui.queuePaused)

	coord.paused = true
	changed = ui.syncPromptQueue()
	require.True(t, changed)
	require.True(t, ui.queuePaused)
}

func TestCancelAgentRequiresSecondEscapeToCancel(t *testing.T) {
	t.Parallel()

	coord := &mockQueueCoordinator{busy: true}
	ui := &UI{
		session: &session.Session{ID: "s1"},
		com:     &common.Common{App: &app.App{AgentCoordinator: coord}},
	}

	cmd := ui.cancelAgent()
	require.NotNil(t, cmd)
	require.True(t, ui.isCanceling)
	require.Empty(t, coord.cancelSessions)

	cmd = ui.cancelAgent()
	require.Nil(t, cmd)
	require.False(t, ui.isCanceling)
	require.Equal(t, []string{"s1"}, coord.cancelSessions)
}

func TestLoadSessionMsgResetsCancelState(t *testing.T) {
	t.Parallel()

	coord := &mockQueueCoordinator{}
	theme := styles.DefaultStyles()
	com := &common.Common{
		App:    &app.App{AgentCoordinator: coord},
		Styles: &theme,
	}
	ui := &UI{
		com:      com,
		chat:     NewChat(com),
		status:   NewStatus(com, nil),
		textarea: textarea.New(),
		session: &session.Session{
			ID: "old-session",
		},
		isCanceling:    true,
		todoIsSpinning: true,
	}

	_, cmd := ui.Update(loadSessionMsg{
		session: &session.Session{ID: "new-session"},
	})
	_ = cmd
	require.False(t, ui.isCanceling)
	require.False(t, ui.todoIsSpinning)
	require.Equal(t, "new-session", ui.session.ID)
}

func TestLoadSessionMsgPrefillsHandoffDraft(t *testing.T) {
	t.Parallel()

	coord := &mockQueueCoordinator{}
	theme := styles.DefaultStyles()
	com := &common.Common{
		App:    &app.App{AgentCoordinator: coord},
		Styles: &theme,
	}
	ui := &UI{
		com:      com,
		chat:     NewChat(com),
		status:   NewStatus(com, nil),
		textarea: textarea.New(),
	}

	_, cmd := ui.Update(loadSessionMsg{
		session: &session.Session{
			ID:                 "handoff-session",
			Kind:               session.KindHandoff,
			HandoffDraftPrompt: "Continue the handoff draft.",
		},
	})
	_ = cmd

	require.Equal(t, "Continue the handoff draft.", ui.textarea.Value())
}
