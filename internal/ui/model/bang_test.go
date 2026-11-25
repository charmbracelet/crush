package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/stretchr/testify/require"
)

// storedBangCommand builds the stored copy of a '!' command run in s1.
func storedBangCommand(id, command string) message.Message {
	return message.Message{
		ID:        id,
		Role:      message.User,
		SessionID: "s1",
		Parts:     []message.ContentPart{message.ShellCommand{Command: command, Output: "hi\n"}},
	}
}

// A '!' command another client ran on the session shows up here too.
func TestBangCommandRunElsewhereShows(t *testing.T) {
	t.Parallel()

	m := newBusyUI(&countingWorkspace{ready: true})
	m.appendSessionMessage(storedBangCommand("m1", "echo hi"))

	ids := transcriptIDs(m)
	require.Len(t, ids, 1)
	require.IsType(t, &chat.ShellItem{}, m.chat.MessageItem(ids[0]))
}

// The UI that ran a '!' command shows it live, so the stored copy must not
// show it again, whether it lands before or after the result. The same
// command run again elsewhere still shows.
func TestBangCommandRunHereShowsOnce(t *testing.T) {
	t.Parallel()

	for name, storedFirst := range map[string]bool{"stored first": true, "result first": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newBusyUI(&countingWorkspace{ready: true})
			m.runShellCommand("echo hi")
			t.Cleanup(m.bangCancel)
			live := transcriptIDs(m)
			require.Len(t, live, 1)

			result := shellResultMsg{PendingID: live[0], Command: "echo hi", Output: "hi\n"}
			if storedFirst {
				m.appendSessionMessage(storedBangCommand("m1", "echo hi"))
				m.Update(result)
			} else {
				m.Update(result)
				m.appendSessionMessage(storedBangCommand("m1", "echo hi"))
			}
			require.Equal(t, live, transcriptIDs(m))

			m.appendSessionMessage(storedBangCommand("m2", "echo hi"))
			require.Len(t, transcriptIDs(m), 2)
		})
	}
}
