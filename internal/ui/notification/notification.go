// Package notification provides desktop notification support for the UI.
package notification

import tea "charm.land/bubbletea/v2"

// Notification represents a desktop notification request.
type Notification struct {
	Title   string
	Message string
}

// Backend defines the interface for sending desktop notifications. Each
// backend controls its own execution model. Implementations are pure transport 
// - policy decisions (config, focus state) are handled by the caller.
type Backend interface {
	Send(n Notification) tea.Cmd
}
