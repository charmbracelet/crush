package notification

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// NativeBackend sends desktop notifications using the native OS notification
// system. Ultra no longer ships a platform-specific implementation; this
// injectable compatibility backend is retained for callers and tests while
// runtime selection falls back to terminal-native notifications.
type NativeBackend struct {
	// icon is the notification icon data (PNG bytes).
	icon []byte
	// notifyFunc is the function used to send notifications (swappable for testing).
	notifyFunc func(title, message string, icon any) error
}

// NewNativeBackend creates a new native notification backend.
func NewNativeBackend(icon []byte) *NativeBackend {
	return &NativeBackend{
		icon:       icon,
		notifyFunc: defaultNotifyFunc,
	}
}

// Send returns a command that sends a desktop notification using the native
// OS notification system.
func (b *NativeBackend) Send(n Notification) tea.Cmd {
	return func() tea.Msg {
		slog.Debug("Sending native notification", "title", n.Title, "message", n.Message)

		if err := b.notifyFunc(n.Title, n.Message, b.icon); err != nil {
			slog.Error("Failed to send notification", "error", err)
		} else {
			slog.Debug("Notification sent successfully")
		}

		return nil
	}
}

// SetNotifyFunc allows replacing the notification function for testing.
func (b *NativeBackend) SetNotifyFunc(fn func(title, message string, icon any) error) {
	b.notifyFunc = fn
}

// ResetNotifyFunc resets the notification function to the default.
func (b *NativeBackend) ResetNotifyFunc() {
	b.notifyFunc = defaultNotifyFunc
}
