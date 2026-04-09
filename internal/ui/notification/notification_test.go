package notification_test

import (
	"encoding/base64"
	"fmt"
	"strings"
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

func TestOSCBackend_Send(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil)

	cmd := backend.Send(notification.Notification{
		Title:   "Crush is waiting...",
		Message: "Agent's turn completed",
	})
	require.NotNil(t, cmd)

	msg := cmd()
	raw, ok := msg.(tea.RawMsg)
	require.True(t, ok)

	s, ok := raw.Msg.(string)
	require.True(t, ok)

	require.Contains(t, s, "Crush is waiting...")
	require.Contains(t, s, "Agent's turn completed")
	require.Contains(t, s, "p=title")
	require.Contains(t, s, "p=body")
	require.NotContains(t, s, "p=icon")
}

func TestOSCBackend_Send_TitleOnly(t *testing.T) {
	t.Parallel()

	backend := notification.NewOSCBackend(nil)

	cmd := backend.Send(notification.Notification{
		Title: "Crush is waiting...",
	})
	require.NotNil(t, cmd)

	msg := cmd()
	raw, ok := msg.(tea.RawMsg)
	require.True(t, ok)

	s, ok := raw.Msg.(string)
	require.True(t, ok)

	require.Contains(t, s, "Crush is waiting...")
	require.Contains(t, s, "p=title")
	require.NotContains(t, s, "p=body")
}

func TestOSCBackend_Send_WithIcon(t *testing.T) {
	t.Parallel()

	iconData := []byte("fake-png-data")
	backend := notification.NewOSCBackend(iconData)

	cmd := backend.Send(notification.Notification{
		Title:   "Test",
		Message: "With icon",
	})
	require.NotNil(t, cmd)

	msg := cmd()
	raw, ok := msg.(tea.RawMsg)
	require.True(t, ok)

	s, ok := raw.Msg.(string)
	require.True(t, ok)

	require.Contains(t, s, "p=icon")
	require.Contains(t, s, "e=1")

	encoded := base64.StdEncoding.EncodeToString(iconData)
	expected := fmt.Sprintf("\x1b]99;i=crush-notify:d=0:p=icon:e=1;%s\x07", encoded)
	require.True(t, strings.Contains(s, expected))
}
