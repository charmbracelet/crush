package notification

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"time"
)

// Notifier handles sending native notifications
type Notifier struct {
	enabled bool
}

// New creates a new Notifier instance
func New(enabled bool) *Notifier {
	return &Notifier{
		enabled: enabled,
	}
}

// NotifyTaskComplete sends a notification when a task is completed
func (n *Notifier) NotifyTaskComplete(ctx context.Context, title, message string) {
	if !n.enabled {
		slog.Debug("Notifications disabled, skipping notification")
		return
	}

	slog.Debug("Sending notification", "title", title, "message", message)
	go func() {
		if err := n.sendNotification(ctx, title, message); err != nil {
			slog.Warn("Failed to send notification", "error", err, "title", title, "message", message)
		} else {
			slog.Debug("Notification sent successfully", "title", title)
		}
	}()
}

// sendNotification sends a platform-specific notification
func (n *Notifier) sendNotification(ctx context.Context, title, message string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "darwin":
		return n.sendMacOSNotification(ctx, title, message)
	case "linux":
		return n.sendLinuxNotification(ctx, title, message)
	case "windows":
		return n.sendWindowsNotification(ctx, title, message)
	default:
		return fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}
}

// sendMacOSNotification sends a notification on macOS using osascript
func (n *Notifier) sendMacOSNotification(ctx context.Context, title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s" sound name "Glass"`, message, title)
	slog.Debug("Executing osascript", "script", script)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("osascript failed", "error", err, "output", string(output))
		return fmt.Errorf("osascript failed: %w, output: %s", err, string(output))
	}

	slog.Debug("osascript succeeded", "output", string(output))
	return nil
}

// sendLinuxNotification sends a notification on Linux using notify-send
func (n *Notifier) sendLinuxNotification(ctx context.Context, title, message string) error {
	cmd := exec.CommandContext(ctx, "notify-send", title, message)
	return cmd.Run()
}

// sendWindowsNotification sends a notification on Windows using multiple methods
func (n *Notifier) sendWindowsNotification(ctx context.Context, title, message string) error {
	slog.Debug("Attempting Windows toast notification")
	// Try PowerShell toast notification first (Windows 10+)
	if err := n.sendWindowsToastNotification(ctx, title, message); err == nil {
		slog.Debug("Windows toast notification succeeded")
		return nil
	} else {
		slog.Debug("Windows toast notification failed, trying fallback", "error", err)
	}

	// Fallback to msg command (works on all Windows versions)
	slog.Debug("Attempting Windows msg notification")
	if err := n.sendWindowsMsgNotification(ctx, title, message); err == nil {
		slog.Debug("Windows msg notification succeeded")
		return nil
	} else {
		slog.Debug("Windows msg notification failed", "error", err)
		return err
	}
}

// sendWindowsToastNotification sends a toast notification using PowerShell (Windows 10+)
func (n *Notifier) sendWindowsToastNotification(ctx context.Context, title, message string) error {
	script := fmt.Sprintf(`
		try {
			[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
			[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
			[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null

			$template = @"
<toast>
	<visual>
		<binding template="ToastText02">
			<text id="1">%s</text>
			<text id="2">%s</text>
		</binding>
	</visual>
</toast>
"@

			$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
			$xml.LoadXml($template)
			$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
			[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Crush").Show($toast)
		} catch {
			exit 1
		}
	`, title, message)

	cmd := exec.CommandContext(ctx, "powershell", "-WindowStyle", "Hidden", "-Command", script)
	return cmd.Run()
}

// sendWindowsMsgNotification sends a notification using msg command (fallback)
func (n *Notifier) sendWindowsMsgNotification(ctx context.Context, title, message string) error {
	// Use msg command to show a message box (works on all Windows versions)
	fullMessage := fmt.Sprintf("%s: %s", title, message)
	cmd := exec.CommandContext(ctx, "msg", "*", fullMessage)
	return cmd.Run()
}
