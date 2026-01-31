package notification_test

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/stretchr/testify/require"
)

func TestNoopBackend_Send(t *testing.T) {
	t.Parallel()

	backend := notification.NoopBackend{}
	err := backend.Send(notification.Notification{
		Title:   "Test Title",
		Message: "Test Message",
	})
	require.NoError(t, err)
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

	err := backend.Send(notification.Notification{
		Title:   "Hello",
		Message: "World",
	})
	require.NoError(t, err)
	require.Equal(t, "Hello", capturedTitle)
	require.Equal(t, "World", capturedMessage)
	require.Nil(t, capturedIcon)
}

func TestChannelSink(t *testing.T) {
	t.Parallel()

	ch := make(chan notification.Notification, 1)
	sink := notification.NewChannelSink(ch)

	sink(notification.Notification{
		Title:   "Test",
		Message: "Notification",
	})

	select {
	case n := <-ch:
		require.Equal(t, "Test", n.Title)
		require.Equal(t, "Notification", n.Message)
	default:
		t.Fatal("expected notification in channel")
	}
}

func TestChannelSink_FullChannel(t *testing.T) {
	t.Parallel()

	// Create a full channel (buffer of 1, already has 1 item).
	ch := make(chan notification.Notification, 1)
	ch <- notification.Notification{Title: "First", Message: "First"}

	sink := notification.NewChannelSink(ch)

	// This should not block; it drains the old notification and sends the new.
	sink(notification.Notification{
		Title:   "Second",
		Message: "Second",
	})

	// The second notification should replace the first (drain-before-send).
	n := <-ch
	require.Equal(t, "Second", n.Title)

	select {
	case <-ch:
		t.Fatal("expected channel to be empty")
	default:
		// Expected.
	}
}
