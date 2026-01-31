package notification

import (
	"log/slog"

	"github.com/gen2brain/beeep"
)

// NativeBackend sends desktop notifications using the native OS notification
// system via beeep.
type NativeBackend struct {
	// icon is the notification icon data (platform-specific).
	icon any
	// notifyFunc is the function used to send notifications (swappable for testing).
	notifyFunc func(title, message string, icon any) error
}

// NewNativeBackend creates a new native notification backend.
func NewNativeBackend(icon any) *NativeBackend {
	beeep.AppName = "Crush"
	return &NativeBackend{
		icon:       icon,
		notifyFunc: beeep.Notify,
	}
}

// Send sends a desktop notification using the native OS notification system.
func (b *NativeBackend) Send(n Notification) error {
	slog.Debug("sending native notification", "title", n.Title, "message", n.Message)

	err := b.notifyFunc(n.Title, n.Message, b.icon)
	if err != nil {
		slog.Error("failed to send notification", "error", err)
	} else {
		slog.Debug("notification sent successfully")
	}

	return err
}

// SetNotifyFunc allows replacing the notification function for testing.
func (b *NativeBackend) SetNotifyFunc(fn func(title, message string, icon any) error) {
	b.notifyFunc = fn
}

// ResetNotifyFunc resets the notification function to the default.
func (b *NativeBackend) ResetNotifyFunc() {
	b.notifyFunc = beeep.Notify
}
