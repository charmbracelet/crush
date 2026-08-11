package notification

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// NativeBackend sends desktop notifications using the native OS notification
// system. The actual delivery function is supplied per-platform via
// defaultNotifyFunc; on illumos/solaris (where beeep's dbus dependency does
// not build) it is a no-op. Selection logic avoids this backend there and
// uses a terminal-based backend instead, so this is only a safety net. See
// NativeSupported.
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
