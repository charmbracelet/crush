package model

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

type mockQueueCoordinator struct {
	queue  int
	paused bool
}

func (m *mockQueueCoordinator) Run(context.Context, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}
func (m *mockQueueCoordinator) Cancel(string)                       {}
func (m *mockQueueCoordinator) CancelAll()                          {}
func (m *mockQueueCoordinator) IsSessionBusy(string) bool           { return false }
func (m *mockQueueCoordinator) IsBusy() bool                        { return false }
func (m *mockQueueCoordinator) QueuedPrompts(string) int            { return m.queue }
func (m *mockQueueCoordinator) QueuedPromptsList(string) []string   { return nil }
func (m *mockQueueCoordinator) RemoveQueuedPrompt(string, int) bool { return false }
func (m *mockQueueCoordinator) ClearQueue(string)                   {}
func (m *mockQueueCoordinator) PauseQueue(string)                   {}
func (m *mockQueueCoordinator) ResumeQueue(string)                  {}
func (m *mockQueueCoordinator) IsQueuePaused(string) bool           { return m.paused }
func (m *mockQueueCoordinator) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}
func (m *mockQueueCoordinator) Model() agent.Model                 { return agent.Model{} }
func (m *mockQueueCoordinator) UpdateModels(context.Context) error { return nil }

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
