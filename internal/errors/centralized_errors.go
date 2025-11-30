// Package errors provides centralized error definitions and handling utilities for Crush.
// This package ensures consistent error messages, types, and handling patterns throughout the application.
package errors

import (
	"errors"
	"fmt"
)

// =================================================================================================
// BASE ERROR CONSTANTS
// =================================================================================================

// Validation Errors - Used for input validation failures
var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrMissingRequired = errors.New("missing required field")
	ErrInvalidFormat   = errors.New("invalid format")
	ErrEmptyInput      = errors.New("empty input")
	ErrInputTooLong    = errors.New("input too long")
	ErrInputTooShort   = errors.New("input too short")
)

// Permission and Authorization Errors
var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrUnauthorized     = errors.New("unauthorized access")
	ErrUserDenied       = errors.New("user denied permission")
)

// Session Management Errors
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionMissing   = errors.New("session id is missing")
	ErrSessionExpired   = errors.New("session expired")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrRequestCancelled = errors.New("request canceled by user")
)

// Tool and Command Errors
var (
	ErrToolNotFound       = errors.New("tool not found")
	ErrToolCallFailed     = errors.New("tool call failed")
	ErrToolCallIDEmpty    = errors.New("tool call ID cannot be empty")
	ErrCommandNotFound    = errors.New("command not found")
	ErrCommandFailed      = errors.New("command execution failed")
	ErrUnsupportedCommand = errors.New("unsupported command")
)

// File System and I/O Errors
var (
	ErrFileNotFound      = errors.New("file not found")
	ErrFileAccessDenied  = errors.New("file access denied")
	ErrFileTooLarge      = errors.New("file too large")
	ErrFileAlreadyExists = errors.New("file already exists")
	ErrDirectoryNotFound = errors.New("directory not found")
	ErrInvalidPath       = errors.New("invalid path")
)

// State and Enum Errors
var (
	ErrToolCallStateUnknown   = errors.New("unknown tool call state")
	ErrAnimationStateUnknown  = errors.New("unknown animation state")
	ErrToolResultStateUnknown = errors.New("unknown tool result state")
	ErrInvalidState           = errors.New("invalid state")
	ErrStateTransition        = errors.New("invalid state transition")
)

// Configuration and Setup Errors
var (
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrMissingConfig    = errors.New("missing configuration")
	ErrConfigLoadFailed = errors.New("failed to load configuration")
	ErrProviderNotFound = errors.New("provider not found")
	ErrProviderFailed   = errors.New("provider initialization failed")
)

// Network and Communication Errors
var (
	ErrConnectionFailed   = errors.New("connection failed")
	ErrTimeout            = errors.New("operation timed out")
	ErrRateLimited        = errors.New("rate limit exceeded")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrInvalidResponse    = errors.New("invalid response received")
)

// Database and Transaction Errors
var (
	ErrTransactionBegin    = errors.New("failed to begin transaction")
	ErrTransactionCommit   = errors.New("failed to commit transaction")
	ErrTransactionRollback = errors.New("failed to rollback transaction")
	ErrDatabaseQuery       = errors.New("database query failed")
	ErrDatabaseConnection  = errors.New("database connection failed")
)

// =================================================================================================
// STRUCTURED ERROR TYPES
// =================================================================================================

// ValidationError provides detailed information about validation failures
type ValidationError struct {
	Field   string // The field that failed validation
	Value   any    // The value that was provided
	Message string // Specific validation message
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

// PermissionError provides context about permission-related failures
type PermissionError struct {
	Resource string // The resource being accessed
	Action   string // The action being attempted
	Session  string // The session ID involved
	Message  string // Additional context
}

func (e *PermissionError) Error() string {
	if e.Resource != "" && e.Action != "" {
		return fmt.Sprintf("permission denied: cannot %s %s", e.Action, e.Resource)
	}
	return ErrPermissionDenied.Error()
}

// ToolError provides detailed information about tool-related failures
type ToolError struct {
	ToolName string // Name of the tool that failed
	Action   string // Action being performed
	Cause    error  // Underlying error
}

func (e *ToolError) Error() string {
	if e.ToolName != "" && e.Action != "" {
		return fmt.Sprintf("tool '%s' failed to %s: %v", e.ToolName, e.Action, e.Cause)
	}
	if e.ToolName != "" {
		return fmt.Sprintf("tool '%s' error: %v", e.ToolName, e.Cause)
	}
	return fmt.Sprintf("tool error: %v", e.Cause)
}

func (e *ToolError) Unwrap() error {
	return e.Cause
}

// StateError represents errors related to state management and transitions
type StateError struct {
	Current     string // Current state
	Target      string // Target state
	Operation   string // Operation being performed
	Description string // Error description
}

func (e *StateError) Error() string {
	if e.Current != "" && e.Target != "" {
		return fmt.Sprintf("state error: cannot transition from '%s' to '%s' during %s: %s",
			e.Current, e.Target, e.Operation, e.Description)
	}
	return fmt.Sprintf("state error: %s", e.Description)
}

// ConfigError represents configuration-related errors
type ConfigError struct {
	Key         string // Configuration key involved
	Value       any    // The problematic value
	Expected    string // Expected format or type
	Description string // Additional context
}

func (e *ConfigError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("configuration error for key '%s': %s", e.Key, e.Description)
	}
	return fmt.Sprintf("configuration error: %s", e.Description)
}

// =================================================================================================
// ERROR CREATION HELPER FUNCTIONS
// =================================================================================================

// Validation creates a standardized validation error
func Validation(field string, value any, message string) error {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// Permission creates a standardized permission error with context
func Permission(resource, action, session string) error {
	return &PermissionError{
		Resource: resource,
		Action:   action,
		Session:  session,
	}
}

// PermissionWithMessage creates a permission error with additional message
func PermissionWithMessage(resource, action, session, message string) error {
	return &PermissionError{
		Resource: resource,
		Action:   action,
		Session:  session,
		Message:  message,
	}
}

// Tool creates a standardized tool error with context
func Tool(toolName, action string, cause error) error {
	return &ToolError{
		ToolName: toolName,
		Action:   action,
		Cause:    cause,
	}
}

// ToolSimple creates a simple tool error without action context
func ToolSimple(toolName string, cause error) error {
	return &ToolError{
		ToolName: toolName,
		Cause:    cause,
	}
}

// State creates a standardized state transition error
func State(current, target, operation, description string) error {
	return &StateError{
		Current:     current,
		Target:      target,
		Operation:   operation,
		Description: description,
	}
}

// Config creates a standardized configuration error
func Config(key string, value any, expected, description string) error {
	return &ConfigError{
		Key:         key,
		Value:       value,
		Expected:    expected,
		Description: description,
	}
}

// ConfigSimple creates a simple configuration error without detailed field info
func ConfigSimple(description string) error {
	return fmt.Errorf("configuration error: %s", description)
}

// File creates a standardized file-related error
func File(operation, path string, cause error) error {
	if path != "" {
		return fmt.Errorf("file error during %s at %s: %w", operation, path, cause)
	}
	return fmt.Errorf("file error during %s: %w", operation, cause)
}

// FileSimple creates a simple file error without path context
func FileSimple(operation string, cause error) error {
	return fmt.Errorf("file error during %s: %w", operation, cause)
}

// Session creates a standardized session-related error
func Session(description string) error {
	return fmt.Errorf("session error: %s", description)
}

// Provider creates a standardized provider-related error
func Provider(description, providerID string) error {
	if providerID != "" {
		return fmt.Errorf("provider error '%s': %s", providerID, description)
	}
	return fmt.Errorf("provider error: %s", description)
}

// ProviderWithCause creates a provider error with underlying cause
func ProviderWithCause(description, providerID string, cause error) error {
	if providerID != "" {
		return fmt.Errorf("provider error '%s': %s: %w", providerID, description, cause)
	}
	return fmt.Errorf("provider error: %s: %w", description, cause)
}

// Transaction creates a standardized transaction-related error
func Transaction(operation string, cause error) error {
	return fmt.Errorf("transaction error during %s: %w", operation, cause)
}

// TransactionBegin creates a standardized transaction begin error
func TransactionBegin(cause error) error {
	return fmt.Errorf("failed to begin transaction: %w", cause)
}

// TransactionCommit creates a standardized transaction commit error
func TransactionCommit(cause error) error {
	return fmt.Errorf("failed to commit transaction: %w", cause)
}

// TransactionRollback creates a standardized transaction rollback error
func TransactionRollback(cause error) error {
	return fmt.Errorf("failed to rollback transaction: %w", cause)
}

// Database creates a standardized database-related error
func Database(operation string, cause error) error {
	return fmt.Errorf("database error during %s: %w", operation, cause)
}

// =================================================================================================
// ERROR WRAPPING AND CONTEXT HELPERS
// =================================================================================================

// WithContext adds context to any error with consistent formatting
func WithContext(err error, operation, context string) error {
	if err == nil {
		return nil
	}
	if context != "" {
		return fmt.Errorf("%s failed: %w: %s", operation, err, context)
	}
	return fmt.Errorf("%s failed: %w", operation, err)
}

// WithField adds field-specific context to an error
func WithField(err error, field string, value any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("field '%s' with value '%v': %w", field, value, err)
}

// Wrap wraps an error with additional context using consistent formatting
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// =================================================================================================
// ERROR CATEGORIZATION AND TESTING
// =================================================================================================

// IsValidationError checks if an error is a ValidationError or wraps one
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// IsPermissionError checks if an error is a PermissionError or wraps one
func IsPermissionError(err error) bool {
	var permissionErr *PermissionError
	return errors.As(err, &permissionErr) ||
		errors.Is(err, ErrPermissionDenied) ||
		errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrUserDenied)
}

// IsToolError checks if an error is a ToolError or wraps one
func IsToolError(err error) bool {
	var toolErr *ToolError
	return errors.As(err, &toolErr)
}

// IsStateError checks if an error is a StateError or wraps one
func IsStateError(err error) bool {
	var stateErr *StateError
	return errors.As(err, &stateErr)
}

// IsConfigError checks if an error is a ConfigError or wraps one
func IsConfigError(err error) bool {
	var configErr *ConfigError
	return errors.As(err, &configErr)
}

// IsRetryable determines if an error represents a condition that might be resolved by retrying
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for retryable base errors
	retryableErrors := []error{
		ErrTimeout,
		ErrConnectionFailed,
		ErrRateLimited,
		ErrServiceUnavailable,
		ErrSessionBusy,
	}

	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	// Check structured error types
	var toolErr *ToolError
	if errors.As(err, &toolErr) {
		// Tool errors might be retryable if they're connection/timeout related
		return IsRetryable(toolErr.Cause)
	}

	return false
}

// IsPermanent determines if an error represents a permanent failure condition
func IsPermanent(err error) bool {
	if err == nil {
		return false
	}

	// Permanent errors typically shouldn't be retried
	permanentErrors := []error{
		ErrPermissionDenied,
		ErrUnauthorized,
		ErrUserDenied,
		ErrInvalidInput,
		ErrMissingRequired,
		ErrInvalidFormat,
		ErrToolNotFound,
		ErrFileNotFound,
		ErrInvalidPath,
	}

	for _, permanentErr := range permanentErrors {
		if errors.Is(err, permanentErr) {
			return true
		}
	}

	// Structured permanent errors
	if IsValidationError(err) || IsPermissionError(err) || IsConfigError(err) {
		return true
	}

	return false
}
