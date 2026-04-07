package model

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	agentnotify "github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

type memoryDreamCoordinator struct {
	mockQueueCoordinator
	status  agent.MemoryFreshnessStatus
	dreamed []string
	forced  []bool
}

func (m *memoryDreamCoordinator) Dream(_ context.Context, sessionID string, force bool) error {
	m.dreamed = append(m.dreamed, sessionID)
	m.forced = append(m.forced, force)
	return nil
}

func (m *memoryDreamCoordinator) MemoryFreshness(context.Context) (agent.MemoryFreshnessStatus, error) {
	return m.status, nil
}

func TestRefreshEditorPlaceholderUsesMemoryFreshness(t *testing.T) {

	ui := testExecutionModeUI(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)
	coord := &memoryDreamCoordinator{}
	coord.status.Warning = "Memory stale: last consolidated 47 days ago — run /dream"
	ui.com.App.AgentCoordinator = coord
	ui.session = &session.Session{ID: "s1", CollaborationMode: session.CollaborationModeDefault}

	ui.refreshMemoryFreshnessNote()
	ui.refreshEditorPlaceholder()

	require.Equal(t, coord.status.Warning, ui.textarea.Placeholder)

	coord.busy = true
	ui.refreshEditorPlaceholder()
	require.Equal(t, ui.workingPlaceholder, ui.textarea.Placeholder)
}

func TestHandleAgentNotificationMemoryDreamLifecycle(t *testing.T) {

	ui := testExecutionModeUI(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)
	backend := &testNotificationBackend{}
	ui.notifyBackend = backend
	ui.notifyWindowFocused = false
	ui.caps.ReportFocusEvents = true
	coord := &memoryDreamCoordinator{}
	ui.com.App.AgentCoordinator = coord
	ui.session = &session.Session{ID: "session-1", Title: "demo"}

	cmd := ui.handleAgentNotification(agentnotify.Notification{SessionID: "session-1", SessionTitle: "demo", Type: agentnotify.TypeMemoryDreamStarted})
	require.NotNil(t, cmd)
	runCmdTree(cmd())
	require.Len(t, backend.notifications, 1)
	require.Equal(t, "Memory dream started", backend.notifications[0].Title)

	cmd = ui.handleAgentNotification(agentnotify.Notification{SessionID: "session-1", SessionTitle: "demo", Type: agentnotify.TypeMemoryDreamFinished})
	require.NotNil(t, cmd)
	runCmdTree(cmd())
	require.Len(t, backend.notifications, 2)
	require.Equal(t, "Memory dream finished", backend.notifications[1].Title)

	cmd = ui.handleAgentNotification(agentnotify.Notification{SessionID: "session-1", SessionTitle: "demo", Type: agentnotify.TypeMemoryDreamFailed})
	require.NotNil(t, cmd)
	runCmdTree(cmd())
	require.Len(t, backend.notifications, 3)
	require.Equal(t, "Memory dream failed", backend.notifications[2].Title)
}

func TestStartMemoryDreamTriggersCoordinator(t *testing.T) {

	coord := &memoryDreamCoordinator{}
	ui := &UI{
		session: &session.Session{ID: "s1"},
		com:     &common.Common{App: &app.App{AgentCoordinator: coord}},
	}

	cmd := ui.startMemoryDream("s1", true)
	require.NotNil(t, cmd)
	msg := cmd()
	started, ok := msg.(memoryDreamStartedMsg)
	require.True(t, ok)
	require.Equal(t, "s1", started.SessionID)
	require.Equal(t, []string{"s1"}, coord.dreamed)
	require.Equal(t, []bool{true}, coord.forced)
}

func TestSendMessageDreamShortcutTriggersCoordinator(t *testing.T) {
	coord := &memoryDreamCoordinator{}
	ui := &UI{
		session: &session.Session{ID: "s1"},
		com:     &common.Common{App: &app.App{AgentCoordinator: coord}},
	}

	cmd := ui.sendMessage("/dream")
	require.NotNil(t, cmd)
	msg := cmd()
	started, ok := msg.(memoryDreamStartedMsg)
	require.True(t, ok)
	require.Equal(t, "s1", started.SessionID)
	require.Equal(t, []string{"s1"}, coord.dreamed)
	require.Equal(t, []bool{true}, coord.forced)
}

func runCmdTree(msg tea.Msg) {
	switch msg := msg.(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, cmd := range msg {
			if cmd != nil {
				runCmdTree(cmd())
			}
		}
	}
}

var _ tea.Msg = memoryDreamStartedMsg{}
var _ = fantasy.ProviderOptions{}
var _ = message.Attachment{}
var _ = permission.AutoClassification{}
