package notification_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/stretchr/testify/require"
)

func TestNoopBackend_Send(t *testing.T) {
	t.Parallel()

	backend := notification.NoopBackend{}
	cmd := backend.Send(notification.Notification{
		Title:   "Test Title",
		Message: "Test Message",
	})
	require.Nil(t, cmd)
}

func TestNativeBackend_Send(t *testing.T) {
	t.Parallel()

	backend := notification.NewNativeBackend(nil)

	var capturedTitle, capturedMessage string
	var capturedIcon any
	backend.SetNotifyFunc(func(title, message string, icon any) error {
		capturedTitle = title
		capturedMessage = message
		capturedIcon = icon
		return nil
	})

	cmd := backend.Send(notification.Notification{
		Title:   "Hello",
		Message: "World",
	})
	require.NotNil(t, cmd)
	msg := cmd()
	require.Nil(t, msg)
	require.Equal(t, "Hello", capturedTitle)
	require.Equal(t, "World", capturedMessage)
	require.Nil(t, capturedIcon)
}

func extractRawString(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	require.NotNil(t, cmd)

	msg := cmd()
	raw, ok := msg.(tea.RawMsg)
	require.True(t, ok)

	s, ok := raw.Msg.(string)
	require.True(t, ok)
	return s
}

func TestOSCBackend_Send_OSC99(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil, true)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title:   "Crush is waiting...",
		Message: "Agent's turn completed",
	}))

	require.Contains(t, s, "p=title")
	require.Contains(t, s, "p=body")
	require.Contains(t, s, "Crush is waiting...")
	require.Contains(t, s, "Agent's turn completed")
	require.NotContains(t, s, "p=icon")
	require.NotContains(t, s, "\x1b]777;")
	require.NotContains(t, s, "\x1b]9;")
}

func TestOSCBackend_Send_OSC99_TitleOnly(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil, true)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title: "Crush is waiting...",
	}))

	require.Contains(t, s, "p=title")
	require.NotContains(t, s, "p=body")
	require.NotContains(t, s, "\x1b]777;")
	require.NotContains(t, s, "\x1b]9;")
}

func TestOSCBackend_Send_OSC99_WithIcon(t *testing.T) {
	t.Parallel()

	iconData := []byte("fake-png-data")
	backend := notification.NewOSCBackend(iconData, true)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title:   "Test",
		Message: "With icon",
	}))

	require.Contains(t, s, "p=icon")
	require.Contains(t, s, "e=1")

	encoded := base64.StdEncoding.EncodeToString(iconData)
	require.Contains(t, s, fmt.Sprintf(";%s\x07", encoded))
	require.NotContains(t, s, "\x1b]777;")
	require.NotContains(t, s, "\x1b]9;")
}

func TestOSCBackend_Send_OSC777(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil, false)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title:   "Test",
		Message: "With body",
	}))

	require.Equal(t, "\x1b]777;notify;Test;With body\x07", s)
	require.NotContains(t, s, "\x1b]99;")
	require.NotContains(t, s, "\x1b]9;")
}

func TestDetectOSC99Support_ValidResponse(t *testing.T) {
	t.Parallel()

	// Simulate a valid OSC 99 response with title support.
	seq := "\x1b]99;i=crush-osc99-query:p=?;p=title\x07"
	require.True(t, notification.DetectOSC99Support(seq))
}

func TestDetectOSC99Support_MultipleCapabilities(t *testing.T) {
	t.Parallel()

	// Response indicating support for title, body, and icon.
	seq := "\x1b]99;i=crush-osc99-query:p=?;p=title,body,icon\x07"
	require.True(t, notification.DetectOSC99Support(seq))
}

func TestDetectOSC99Support_InvalidCommand(t *testing.T) {
	t.Parallel()

	// OSC 98 instead of 99.
	seq := "\x1b]98;i=crush-osc99-query:p=?;p=title\x07"
	require.False(t, notification.DetectOSC99Support(seq))
}

func TestDetectOSC99Support_WrongQueryID(t *testing.T) {
	t.Parallel()

	// Correct OSC 99 but wrong query ID.
	seq := "\x1b]99;i=some-other-id:p=?;p=title\x07"
	require.False(t, notification.DetectOSC99Support(seq))
}

func TestDetectOSC99Support_NoQueryFlag(t *testing.T) {
	t.Parallel()

	// Missing p=? query flag.
	seq := "\x1b]99;i=crush-osc99-query;p=title\x07"
	require.False(t, notification.DetectOSC99Support(seq))
}

func TestDetectOSC99Support_NoTitleCapability(t *testing.T) {
	t.Parallel()

	// Response without title capability (only body).
	seq := "\x1b]99;i=crush-osc99-query:p=?;p=body\x07"
	require.False(t, notification.DetectOSC99Support(seq))
}

func TestDetectOSC99Support_EmptySequence(t *testing.T) {
	t.Parallel()

	require.False(t, notification.DetectOSC99Support(""))
}

func TestDetectOSC99Support_MalformedSequence(t *testing.T) {
	t.Parallel()

	// Missing semicolon separator.
	seq := "\x1b]99;i=crush-osc99-query:p=?p=title\x07"
	require.False(t, notification.DetectOSC99Support(seq))
}

func TestOSC99QuerySequence(t *testing.T) {
	t.Parallel()

	seq := notification.OSC99QuerySequence()
	require.Contains(t, seq, "\x1b]99;")
	require.Contains(t, seq, "i=crush-osc99-query")
	require.Contains(t, seq, "p=?")
	require.Contains(t, seq, "\x07")
}

func TestBellBackend_Send(t *testing.T) {
	t.Parallel()

	backend := notification.NewBellBackend()
	s := extractRawString(t, backend.Send(notification.Notification{
		Title:   "Test",
		Message: "Ignored by bell",
	}))

	// Bell backend only sends the bell character.
	require.Equal(t, "\x07", s)
	require.NotContains(t, s, "Test")
	require.NotContains(t, s, "Ignored")
}
