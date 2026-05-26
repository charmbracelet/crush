// Package notification provides desktop notification support for the UI.
//
// This package supports multiple notification backends:
//   - NativeBackend: Uses the native OS notification system (macOS, Windows, Linux)
//   - OSCBackend: Uses OSC escape sequences with automatic protocol detection.
//     Prefers OSC 99 (modern standard with rich notifications) if supported,
//     falling back to OSC 777 (urxvt extension, widely supported). Used for SSH sessions.
//   - BellBackend: Triggers the terminal bell character (\x07), causing an audible
//     beep or visual flash. Works in virtually all terminals but provides no message text.
//   - NoopBackend: A no-op backend that silently discards notifications. Used when
//     notifications are disabled or no suitable backend is available.
//
// Backend selection is based on terminal capabilities, environment, and user config:
//   - Users can explicitly set notification_style in crush.json (auto/native/osc/bell/disabled)
//   - Auto mode: SSH sessions use OSC backend (auto-detects OSC 99 vs 777)
//   - Auto mode: Local sessions use native OS notifications
//   - If focus events are not supported in local sessions, notifications are disabled (NoopBackend)
package notification

import tea "charm.land/bubbletea/v2"

// Notification represents a desktop notification request.
type Notification struct {
	Title   string
	Message string
}

// Backend defines the interface for sending desktop notifications.
// Implementations return a tea.Cmd that performs the notification, allowing
// each backend to choose between synchronous (native OS) and asynchronous
// (terminal escape sequences) delivery. Policy decisions (config checks,
// focus state) are handled by the caller.
type Backend interface {
	Send(n Notification) tea.Cmd
}
