//go:build darwin

package notification

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
)

// terminalBundleIDs maps TERM_PROGRAM values to macOS bundle identifiers.
var terminalBundleIDs = map[string]string{
	"Apple_Terminal": "com.apple.Terminal",
	"ghostty":        "com.mitchellh.ghostty",
	"iTerm.app":      "com.googlecode.iterm2",
	"WezTerm":        "org.wezfurlong.wezterm",
	"kitty":          "net.kovidgoyal.kitty",
	"Alacritty":      "org.alacritty",
	"vscode":         "com.microsoft.VSCode",
	"tmux":           "", // tmux runs inside another terminal
	"screen":         "", // screen runs inside another terminal
}

// NativeBackend sends desktop notifications using terminal-notifier on macOS,
// with the ability to focus the originating terminal when clicked.
type NativeBackend struct {
	// terminalBundleID is the bundle ID of the terminal app to activate on click.
	terminalBundleID string
	// hasTerminalNotifier indicates if terminal-notifier is available.
	hasTerminalNotifier bool
	// icon is the notification icon data (unused on darwin, kept for interface compat).
	icon any
	// notifyFunc is the fallback function used when terminal-notifier is unavailable.
	notifyFunc func(title, message string, icon any) error
}

// NewNativeBackend creates a new native notification backend for macOS.
// It detects the current terminal and configures notifications to focus it when clicked.
func NewNativeBackend(icon any) *NativeBackend {
	beeep.AppName = "Crush"

	b := &NativeBackend{
		icon:       icon,
		notifyFunc: beeep.Notify,
	}

	// Check if terminal-notifier is available.
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		b.hasTerminalNotifier = true
		slog.Debug("Found terminal-notifier", "path", path)
	}

	// Detect the terminal bundle ID from TERM_PROGRAM.
	if termProgram := os.Getenv("TERM_PROGRAM"); termProgram != "" {
		if bundleID, ok := terminalBundleIDs[termProgram]; ok && bundleID != "" {
			b.terminalBundleID = bundleID
			slog.Debug("Detected terminal for notifications", "term_program", termProgram, "bundle_id", bundleID)
		} else if !ok {
			// Unknown terminal - try to look up its bundle ID dynamically.
			if bundleID := lookupBundleID(termProgram); bundleID != "" {
				b.terminalBundleID = bundleID
				slog.Debug("Looked up terminal bundle ID", "term_program", termProgram, "bundle_id", bundleID)
			}
		}
	}

	return b
}

// lookupBundleID attempts to find the bundle ID for an app name using osascript.
func lookupBundleID(appName string) string {
	// Try common variations of the app name.
	variations := []string{appName, strings.TrimSuffix(appName, ".app")}
	for _, name := range variations {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, "osascript", "-e", `id of app "`+name+`"`)
		out, err := cmd.Output()
		cancel()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// Send sends a desktop notification. If terminal-notifier is available and we know
// the terminal's bundle ID, clicking the notification will focus that terminal.
func (b *NativeBackend) Send(n Notification) error {
	slog.Debug("Sending native notification", "title", n.Title, "message", n.Message,
		"has_terminal_notifier", b.hasTerminalNotifier, "terminal_bundle_id", b.terminalBundleID)

	// Use terminal-notifier if available.
	if b.hasTerminalNotifier {
		return b.sendWithTerminalNotifier(n)
	}

	// Fall back to beeep (which uses osascript on darwin).
	err := b.notifyFunc(n.Title, n.Message, b.icon)
	if err != nil {
		slog.Error("Failed to send notification via beeep", "error", err)
	}
	return err
}

// sendWithTerminalNotifier sends a notification using terminal-notifier.
func (b *NativeBackend) sendWithTerminalNotifier(n Notification) error {
	args := []string{
		"-title", n.Title,
		"-message", n.Message,
		"-group", "crush",
	}

	// If we know the terminal, activate it when the notification is clicked.
	if b.terminalBundleID != "" {
		args = append(args, "-activate", b.terminalBundleID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "terminal-notifier", args...)
	if err := cmd.Run(); err != nil {
		slog.Error("Failed to send notification via terminal-notifier", "error", err)
		return err
	}

	slog.Debug("Notification sent via terminal-notifier")
	return nil
}

// SetNotifyFunc allows replacing the fallback notification function for testing.
// This also disables terminal-notifier to ensure the fallback is used.
func (b *NativeBackend) SetNotifyFunc(fn func(title, message string, icon any) error) {
	b.notifyFunc = fn
	b.hasTerminalNotifier = false
}

// ResetNotifyFunc resets the fallback notification function to the default.
func (b *NativeBackend) ResetNotifyFunc() {
	b.notifyFunc = beeep.Notify
}
