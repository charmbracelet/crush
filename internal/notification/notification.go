package notification

import (
	"context"
	"log/slog"
	"time"

	"github.com/gen2brain/beeep"
)

// Notifier handles sending native notifications.
type Notifier struct {
	enabled bool
}

// New creates a new Notifier instance.
func New(enabled bool) *Notifier {
	return &Notifier{
		enabled: enabled,
	}
}

// NotifyTaskComplete sends a notification when a task is completed.
// It waits for the specified delay before sending the notification.
func (n *Notifier) NotifyTaskComplete(ctx context.Context, title, message string, delay time.Duration) context.CancelFunc {
	if !n.enabled {
		slog.Debug("Notifications disabled, skipping completion notification")
		return func() {}
	}

	notifyCtx, cancel := context.WithCancel(ctx)
	go func() {
		slog.Debug("Waiting before sending completion notification", "delay", delay)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			slog.Debug("Sending completion notification", "title", title, "message", message)
			if err := beeep.Notify(title, message, ""); err != nil {
				slog.Warn("Failed to send notification", "error", err, "title", title, "message", message)
			} else {
				slog.Debug("Notification sent successfully", "title", title)
			}
		case <-notifyCtx.Done():
			slog.Debug("Completion notification cancelled")
		}
	}()
	return cancel
}

// NotifyPermissionRequest sends a notification when a permission request needs attention.
// It waits for the specified delay before sending the notification.
func (n *Notifier) NotifyPermissionRequest(ctx context.Context, title, message string, delay time.Duration) context.CancelFunc {
	if !n.enabled {
		slog.Debug("Notifications disabled, skipping permission request notification")
		return func() {}
	}

	notifyCtx, cancel := context.WithCancel(ctx)
	go func() {
		slog.Debug("Waiting before sending permission request notification", "delay", delay)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			slog.Debug("Sending permission request notification", "title", title, "message", message)
			if err := beeep.Notify(title, message, ""); err != nil {
				slog.Warn("Failed to send notification", "error", err, "title", title, "message", message)
			} else {
				slog.Debug("Notification sent successfully", "title", title)
			}
		case <-notifyCtx.Done():
			slog.Debug("Permission request notification cancelled")
		}
	}()
	return cancel
}
