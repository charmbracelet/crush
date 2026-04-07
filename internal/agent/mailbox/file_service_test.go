package mailbox

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileService_GetInboxPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	path := svc.GetInboxPath("agent-1", "team-1")
	expected := filepath.Join(tmpDir, "teams", "team-1", "inboxes", "agent-1.json")
	require.Equal(t, expected, path)
}

func TestFileService_ReadEmpty(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	messages, err := svc.Read("agent-1", "team-1")
	require.NoError(t, err)
	require.Empty(t, messages)
}

func TestFileService_WriteAndRead(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	msg := Message{
		From:    "coordinator",
		Text:    "Hello from coordinator",
		Color:   "#ff0000",
		Summary: "Greeting",
	}

	err := svc.Write("agent-1", "team-1", msg)
	require.NoError(t, err)

	messages, err := svc.Read("agent-1", "team-1")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "coordinator", messages[0].From)
	require.Equal(t, "Hello from coordinator", messages[0].Text)
	require.False(t, messages[0].Read)
	require.NotZero(t, messages[0].Timestamp)
}

func TestFileService_ReadUnread(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	// Write two messages
	err := svc.Write("agent-1", "team-1", Message{From: "sender1", Text: "msg1"})
	require.NoError(t, err)

	err = svc.Write("agent-1", "team-1", Message{From: "sender2", Text: "msg2"})
	require.NoError(t, err)

	// Mark first as read
	err = svc.MarkMessageAsRead("agent-1", "team-1", 0)
	require.NoError(t, err)

	unread, err := svc.ReadUnread("agent-1", "team-1")
	require.NoError(t, err)
	require.Len(t, unread, 1)
	require.Equal(t, "msg2", unread[0].Text)
}

func TestFileService_MarkAsRead(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	err := svc.Write("agent-1", "team-1", Message{From: "sender", Text: "msg"})
	require.NoError(t, err)

	err = svc.MarkAsRead("agent-1", "team-1")
	require.NoError(t, err)

	messages, err := svc.Read("agent-1", "team-1")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.True(t, messages[0].Read)
}

func TestFileService_MarkMessageAsRead(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	err := svc.Write("agent-1", "team-1", Message{From: "sender", Text: "msg1"})
	require.NoError(t, err)

	err = svc.Write("agent-1", "team-1", Message{From: "sender", Text: "msg2"})
	require.NoError(t, err)

	err = svc.MarkMessageAsRead("agent-1", "team-1", 1)
	require.NoError(t, err)

	messages, err := svc.Read("agent-1", "team-1")
	require.NoError(t, err)
	require.False(t, messages[0].Read)
	require.True(t, messages[1].Read)
}

func TestFileService_MarkMessageAsReadInvalidIndex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	err := svc.Write("agent-1", "team-1", Message{From: "sender", Text: "msg"})
	require.NoError(t, err)

	err = svc.MarkMessageAsRead("agent-1", "team-1", 10)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid message index")
}

func TestFileService_Clear(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	err := svc.Write("agent-1", "team-1", Message{From: "sender", Text: "msg"})
	require.NoError(t, err)

	err = svc.Clear("agent-1", "team-1")
	require.NoError(t, err)

	messages, err := svc.Read("agent-1", "team-1")
	require.NoError(t, err)
	require.Empty(t, messages)
}

func TestFileService_MultipleMessages(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := NewFileService(tmpDir)

	for i := 0; i < 5; i++ {
		err := svc.Write("agent-1", "team-1", Message{
			From: "sender",
			Text: "message",
		})
		require.NoError(t, err)
	}

	messages, err := svc.Read("agent-1", "team-1")
	require.NoError(t, err)
	require.Len(t, messages, 5)
}

func TestSanitizeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"simple-name", "simple-name"},
		{"name with spaces", "name-with-spaces"},
		{"path/to/file", "path-to-file"},
		{"colon:test", "colon-test"},
		{"  trimmed  ", "trimmed"},
		{"..", "default"},
		{"../escape", "escape"},
		{"", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
