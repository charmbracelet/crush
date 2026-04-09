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

func TestOSCBackend_Send(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title:   "Crush is waiting...",
		Message: "Agent's turn completed",
	}))

	// OSC 99 (kitty).
	require.Contains(t, s, "p=title")
	require.Contains(t, s, "p=body")
	require.Contains(t, s, "Crush is waiting...")
	require.Contains(t, s, "Agent's turn completed")
	require.NotContains(t, s, "p=icon")

	// OSC 777 (VTE).
	require.Contains(t, s, "\x1b]777;notify;Crush is waiting...;Agent's turn completed\x07")

	// OSC 9 (iTerm2/WezTerm).
	require.Contains(t, s, "\x1b]9;Crush is waiting...: Agent's turn completed\x07")
}

func TestOSCBackend_Send_TitleOnly(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title: "Crush is waiting...",
	}))

	// OSC 99.
	require.Contains(t, s, "p=title")
	require.NotContains(t, s, "p=body")

	// OSC 777 — title with empty body.
	require.Contains(t, s, "\x1b]777;notify;Crush is waiting...;\x07")

	// OSC 9 — title only.
	require.Contains(t, s, "\x1b]9;Crush is waiting...\x07")
}

func TestOSCBackend_Send_WithIcon(t *testing.T) {
	t.Parallel()

	iconData := []byte("fake-png-data")
	backend := notification.NewOSCBackend(iconData)
	s := extractRawString(t, backend.Send(notification.Notification{
		Title:   "Test",
		Message: "With icon",
	}))

	// OSC 99 icon payload.
	require.Contains(t, s, "p=icon")
	require.Contains(t, s, "e=1")

	encoded := base64.StdEncoding.EncodeToString(iconData)
	require.Contains(t, s, fmt.Sprintf("d=0:p=icon:e=1;%s\x07", encoded))

	// OSC 777.
	require.Contains(t, s, "\x1b]777;notify;Test;With icon\x07")

	// OSC 9.
	require.Contains(t, s, "\x1b]9;Test: With icon\x07")
}
