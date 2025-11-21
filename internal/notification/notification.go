package notification

import (
	"log/slog"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/gen2brain/beeep"
)

var (
	isFocused      = true
	supportsFocus  = false
	focusStateLock sync.RWMutex

	// notifyFunc is the function used to send notifications.
	// It can be swapped for testing.
	notifyFunc = beeep.Notify
)

// SetFocusSupport sets whether the terminal supports focus reporting.
func SetFocusSupport(supported bool) {
	focusStateLock.Lock()
	defer focusStateLock.Unlock()
	supportsFocus = supported
}

// SetFocused sets whether the terminal window is currently focused.
func SetFocused(focused bool) {
	focusStateLock.Lock()
	defer focusStateLock.Unlock()
	isFocused = focused
}

// IsFocused returns whether the terminal window is currently focused.
func IsFocused() bool {
	focusStateLock.RLock()
	defer focusStateLock.RUnlock()
	return isFocused
}

// Send sends a desktop notification with the given title and message.
// Notifications are only sent when focus reporting is supported, the terminal window is not focused, and notifications are not disabled in config.
// On darwin (macOS), icons are omitted due to platform limitations.
func Send(title, message string) error {
	// Check if notifications are disabled in config
	cfg := config.Get()
	if cfg != nil && cfg.Options != nil && cfg.Options.DisableNotifications {
		slog.Debug("skipping notification: disabled in config")
		return nil
	}

	focusStateLock.RLock()
	focused := isFocused
	supported := supportsFocus
	focusStateLock.RUnlock()

	slog.Debug("notification.Send called", "title", title, "message", message, "focused", focused, "supported", supported)

	// Only send notifications if focus reporting is supported and window is not focused.
	if !supported || focused {
		slog.Debug("skipping notification: focus not supported or window is focused")
		return nil
	}

	beeep.AppName = "Crush"

	err := notifyFunc(title, message, notificationIcon)

	if err != nil {
		slog.Error("failed to send notification", "error", err)
	} else {
		slog.Debug("notification sent successfully")
	}

	return err
}
