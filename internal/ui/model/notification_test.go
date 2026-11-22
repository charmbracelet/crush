package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/notification"
)

type recordingNotificationBackend struct {
	sent []notification.Notification
}

func (b *recordingNotificationBackend) Send(n notification.Notification) tea.Cmd {
	return func() tea.Msg {
		b.sent = append(b.sent, n)
		return nil
	}
}

func TestAgentFinishedNotificationIsScopedToViewedSession(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		sessionID string
		focused   bool
		landing   bool
		want      int
	}{
		{name: "viewed session in background", sessionID: "s1", want: 1},
		{name: "viewed session in foreground", sessionID: "s1", focused: true},
		{name: "another client's session", sessionID: "s2"},
		{name: "no viewed session", sessionID: "s1", landing: true},
		{name: "no event session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := &countingWorkspace{ready: true}
			m := newBusyUI(ws)
			warmCaches(m, true)
			backend := &recordingNotificationBackend{}
			m.notifyBackend = backend
			m.caps.ReportFocusEvents = true
			m.notifyWindowFocused = tc.focused
			if tc.landing {
				m.session = nil
			}

			_, cmd := m.Update(pubsub.Event[notify.Notification]{
				Type: pubsub.CreatedEvent,
				Payload: notify.Notification{
					Type:         notify.TypeAgentFinished,
					SessionID:    tc.sessionID,
					SessionTitle: "example",
				},
			})
			runCmds(m, cmd)

			require.Len(t, backend.sent, tc.want)
			if tc.want != 0 {
				require.Equal(t, "Agent's turn completed in \"example\"", backend.sent[0].Message)
			}
			require.False(t, m.isAgentBusy(), "shared completion events must still refresh busy state")
		})
	}
}
