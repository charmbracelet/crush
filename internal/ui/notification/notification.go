// Package notification provides desktop notification support for the UI.
package notification

// Notification represents a desktop notification request.
type Notification struct {
	Title   string
	Message string
}

// Backend defines the interface for sending desktop notifications.
// Implementations are pure transport - policy decisions (config, focus state)
// are handled by the caller.
type Backend interface {
	Send(n Notification) error
}
