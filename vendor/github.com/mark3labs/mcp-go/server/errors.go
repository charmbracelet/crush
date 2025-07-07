package server

import (
	"errors"
	"fmt"
)

var (
	// Common server errors
	ErrUnsupported      = errors.New("not supported")
	ErrResourceNotFound = errors.New("resource not found")
	ErrPromptNotFound   = errors.New("prompt not found")
	ErrToolNotFound     = errors.New("tool not found")

	// Session-related errors
	ErrSessionNotFound              = errors.New("session not found")
	ErrSessionExists                = errors.New("session already exists")
	ErrSessionNotInitialized        = errors.New("session not properly initialized")
	ErrSessionDoesNotSupportTools   = errors.New("session does not support per-session tools")
	ErrSessionDoesNotSupportLogging = errors.New("session does not support setting logging level")

	// Notification-related errors
	ErrNotificationNotInitialized = errors.New("notification channel not initialized")
	ErrNotificationChannelBlocked = errors.New("notification channel full or blocked")
)

// ErrDynamicPathConfig is returned when attempting to use static path methods with dynamic path configuration
type ErrDynamicPathConfig struct {
	Method string
}

func (e *ErrDynamicPathConfig) Error() string {
	return fmt.Sprintf("%s cannot be used with WithDynamicBasePath. Use dynamic path logic in your router.", e.Method)
}
