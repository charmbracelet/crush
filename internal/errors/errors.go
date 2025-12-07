package errors

import (
	"fmt"
	"net/http"
)

// ErrorType represents categories of errors for proper handling
type ErrorType string

const (
	// TypeNetwork represents network-related errors
	TypeNetwork ErrorType = "network"
	
	// TypeValidation represents validation errors
	TypeValidation ErrorType = "validation"
	
	// TypeState represents impossible state errors
	TypeState ErrorType = "state"
	
	// TypePermission represents permission errors
	TypePermission ErrorType = "permission"
	
	// TypeConfig represents configuration errors
	TypeConfig ErrorType = "config"
	
	// TypeDatabase represents database errors
	TypeDatabase ErrorType = "database"
	
	// TypeAgent represents agent execution errors
	TypeAgent ErrorType = "agent"
	
	// TypeUI represents UI/TUI errors
	TypeUI ErrorType = "ui"
	
	// TypeTool represents tool execution errors
	TypeTool ErrorType = "tool"
	
	// TypeBDDTest represents BDD test errors
	TypeBDDTest ErrorType = "bdd_test"
)

// CrushError represents a structured error with type and context
type CrushError struct {
	Type        ErrorType                 `json:"type"`
	Message     string                    `json:"message"`
	Code        string                    `json:"code,omitempty"`
	Context     map[string]interface{}    `json:"context,omitempty"`
	Cause       error                     `json:"cause,omitempty"`
	Retryable   bool                      `json:"retryable"`
	UserMessage string                    `json:"user_message,omitempty"`
}

// Error implements the error interface
func (e *CrushError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause
func (e *CrushError) Unwrap() error {
	return e.Cause
}

// IsType checks if error matches specific type
func (e *CrushError) IsType(errorType ErrorType) bool {
	return e.Type == errorType
}

// IsRetryable returns whether error can be retried
func (e *CrushError) IsRetryable() bool {
	return e.Retryable
}

// NewError creates a new typed error
func NewError(errorType ErrorType, message string) *CrushError {
	return &CrushError{
		Type:      errorType,
		Message:   message,
		Retryable: false,
	}
}

// NewErrorWithCause creates a new typed error with cause
func NewErrorWithCause(errorType ErrorType, message string, cause error) *CrushError {
	return &CrushError{
		Type:      errorType,
		Message:   message,
		Cause:     cause,
		Retryable: false,
	}
}

// NewRetryableError creates a retryable typed error
func NewRetryableError(errorType ErrorType, message string) *CrushError {
	return &CrushError{
		Type:      errorType,
		Message:   message,
		Retryable: true,
	}
}

// NewRetryableErrorWithCause creates a retryable typed error with cause
func NewRetryableErrorWithCause(errorType ErrorType, message string, cause error) *CrushError {
	return &CrushError{
		Type:      errorType,
		Message:   message,
		Cause:     cause,
		Retryable: true,
	}
}

// WithCode adds error code
func (e *CrushError) WithCode(code string) *CrushError {
	e.Code = code
	return e
}

// WithContext adds context information
func (e *CrushError) WithContext(key string, value interface{}) *CrushError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithUserMessage adds user-friendly message
func (e *CrushError) WithUserMessage(message string) *CrushError {
	e.UserMessage = message
	return e
}

// MakeRetryable marks error as retryable
func (e *CrushError) MakeRetryable() *CrushError {
	e.Retryable = true
	return e
}

// Error builders for common scenarios

// NetworkError creates network-related errors
func NetworkError(message string) *CrushError {
	return NewError(TypeNetwork, message)
}

// NetworkErrorWithCause creates network error with cause
func NetworkErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeNetwork, message, cause).MakeRetryable()
}

// ValidationError creates validation errors
func ValidationError(message string) *CrushError {
	return NewError(TypeValidation, message)
}

// ValidationErrorWithCause creates validation error with cause
func ValidationErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeValidation, message, cause)
}

// StateError creates impossible state errors (split-brain detection)
func StateError(message string) *CrushError {
	return NewError(TypeState, message)
}

// StateErrorWithCause creates state error with cause
func StateErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeState, message, cause)
}

// PermissionError creates permission errors
func PermissionError(message string) *CrushError {
	return NewError(TypePermission, message)
}

// PermissionErrorWithCause creates permission error with cause
func PermissionErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypePermission, message, cause)
}

// ConfigError creates configuration errors
func ConfigError(message string) *CrushError {
	return NewError(TypeConfig, message)
}

// ConfigErrorWithCause creates config error with cause
func ConfigErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeConfig, message, cause)
}

// DatabaseError creates database errors
func DatabaseError(message string) *CrushError {
	return NewErrorWithCause(TypeDatabase, message, nil).MakeRetryable()
}

// DatabaseErrorWithCause creates database error with cause
func DatabaseErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeDatabase, message, cause).MakeRetryable()
}

// AgentError creates agent execution errors
func AgentError(message string) *CrushError {
	return NewError(TypeAgent, message)
}

// AgentErrorWithCause creates agent error with cause
func AgentErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeAgent, message, cause).MakeRetryable()
}

// UIError creates UI/TUI errors
func UIError(message string) *CrushError {
	return NewError(TypeUI, message)
}

// UIErrorWithCause creates UI error with cause
func UIErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeUI, message, cause)
}

// ToolError creates tool execution errors
func ToolError(message string) *CrushError {
	return NewError(TypeTool, message)
}

// ToolErrorWithCause creates tool error with cause
func ToolErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeTool, message, cause)
}

// BDDTestError creates BDD test errors
func BDDTestError(message string) *CrushError {
	return NewError(TypeBDDTest, message)
}

// BDDTestErrorWithCause creates BDD test error with cause
func BDDTestErrorWithCause(message string, cause error) *CrushError {
	return NewErrorWithCause(TypeBDDTest, message, cause)
}

// HTTP Status code helpers
func (e *CrushError) HTTPStatusCode() int {
	switch e.Type {
	case TypeValidation:
		return http.StatusBadRequest
	case TypePermission:
		return http.StatusForbidden
	case TypeConfig:
		return http.StatusInternalServerError
	case TypeDatabase:
		return http.StatusInternalServerError
	case TypeNetwork:
		return http.StatusServiceUnavailable
	case TypeAgent:
		return http.StatusServiceUnavailable
	case TypeUI:
		return http.StatusInternalServerError
	case TypeTool:
		return http.StatusInternalServerError
	case TypeState:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// Error classification helpers
func IsNetworkError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeNetwork)
	}
	return false
}

func IsValidationError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeValidation)
	}
	return false
}

func IsStateError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeState)
	}
	return false
}

func IsPermissionError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypePermission)
	}
	return false
}

func IsConfigError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeConfig)
	}
	return false
}

func IsDatabaseError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeDatabase)
	}
	return false
}

func IsAgentError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeAgent)
	}
	return false
}

func IsUIError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeUI)
	}
	return false
}

func IsToolError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeTool)
	}
	return false
}

func IsBDDTestError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsType(TypeBDDTest)
	}
	return false
}

func IsRetryableError(err error) bool {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr.IsRetryable()
	}
	return false
}

// ConvertToCrushError converts any error to CrushError with best-guess type
func ConvertToCrushError(err error) *CrushError {
	if crushErr, ok := err.(*CrushError); ok {
		return crushErr
	}
	
	// Guess error type based on common patterns
	errStr := err.Error()
	if contains(errStr, []string{"network", "connection", "timeout", "unreachable"}) {
		return NetworkErrorWithCause("Network error occurred", err)
	}
	if contains(errStr, []string{"invalid", "validation", "missing", "required"}) {
		return ValidationErrorWithCause("Validation error occurred", err)
	}
	if contains(errStr, []string{"database", "sql", "query"}) {
		return DatabaseErrorWithCause("Database error occurred", err)
	}
	if contains(errStr, []string{"permission", "unauthorized", "forbidden"}) {
		return PermissionErrorWithCause("Permission error occurred", err)
	}
	
	// Default to generic error
	return NewErrorWithCause(TypeAgent, "Unknown error occurred", err)
}

// Helper function for string matching
func contains(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}