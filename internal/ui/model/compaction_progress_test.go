package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func sendNote(t *testing.T, m *UI, n notify.Notification) tea.Cmd {
	t.Helper()
	_, cmd := m.Update(pubsub.Event[notify.Notification]{
		Type:    pubsub.CreatedEvent,
		Payload: n,
	})
	return cmd
}

// TestSummarizingProgressDrivesPlaceholder covers the only place a compaction
// shows up while it runs: a model on a local machine can take minutes, and a
// bare spinner does not explain that the session is being rewritten.
func TestSummarizingProgressDrivesPlaceholder(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true, agentBusy: true}
	m := newBusyUI(ws)
	warmCaches(m, true)

	note := "Compacting 40 messages · checkpoint 1.2k/8.2k tokens"
	runCmds(m, sendNote(t, m, notify.Notification{
		Type:      notify.TypeSummarizing,
		SessionID: "s1",
		Progress:  note,
	}))
	require.Equal(t, note, m.summarizingNote)
	require.Equal(t, note, m.textarea.Placeholder)

	// The closing notification clears the note whatever its outcome, so a
	// canceled or failed compaction cannot leave a lie on screen.
	runCmds(m, sendNote(t, m, notify.Notification{
		Type:      notify.TypeSummarizing,
		SessionID: "s1",
		Progress:  "Compacted 40 messages, kept 6 verbatim",
		Done:      true,
	}))
	require.Empty(t, m.summarizingNote)
	require.NotEqual(t, note, m.textarea.Placeholder)
}

// TestSummarizingProgressIgnoresOtherSessions keeps a background session's
// compaction from being described as the focused one.
// TestCompactionVisibleFollowsTheFocusedSession pins the note to the session
// being rewritten: switching away mid-compaction must not describe the session
// the user is now looking at, and switching back must show it again.
func TestCompactionVisibleFollowsTheFocusedSession(t *testing.T) {
	pinTTLs(t)

	m := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(m, true)
	runCmds(m, sendNote(t, m, notify.Notification{
		Type:      notify.TypeSummarizing,
		SessionID: "s1",
		Progress:  "Compacting 40 messages",
	}))
	require.True(t, m.compactionVisible())

	m.session.ID = "s2"
	require.False(t, m.compactionVisible(), "another session's compaction is not this session's status")

	m.session.ID = "s1"
	require.True(t, m.compactionVisible())
}

func TestSummarizingProgressIgnoresOtherSessions(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)

	runCmds(m, sendNote(t, m, notify.Notification{
		Type:      notify.TypeSummarizing,
		SessionID: "some-other-session",
		Progress:  "Compacting 40 messages",
	}))
	require.Empty(t, m.summarizingNote)
}
