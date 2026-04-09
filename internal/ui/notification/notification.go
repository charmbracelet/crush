// Package notification provides desktop notification support for the UI.
//
// This package supports multiple notification backends:
//   - NativeBackend: Uses the native OS notification system (macOS, Windows, Linux)
//   - OSC99Backend: Uses the OSC 99 Desktop Notification protocol, supported by
//     modern terminals like kitty, wezterm, and ghostty. Supports rich notifications
//     with title, body, icons, and actions.
//   - OSC777Backend: Uses the OSC 777 urxvt notification extension, widely supported
//     but less capable (title and body only). Used as a fallback for SSH sessions.
//   - NoopBackend: A no-op backend that silently discards notifications.
//
// Backend selection is based on terminal capabilities and environment:
//   - SSH sessions prefer OSC 99 if available, falling back to OSC 777
//   - Local sessions use native OS notifications
//   - If focus events are not supported, notifications are disabled (NoopBackend)
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
