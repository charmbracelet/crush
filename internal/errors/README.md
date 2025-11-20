# Centralized Error System

This document describes the comprehensive centralized error system implemented in Crush.

## Architecture Overview

The centralized error system provides:
- **Consistent error messages** across all packages
- **Structured error types** for detailed error handling
- **Helper functions** for common error patterns
- **Error categorization** for better error handling logic
- **Backward compatibility** through re-exports

## File Structure

```
internal/errors/
├── centralized_errors.go  # Main error definitions and helpers
└── README.md           # This documentation
```

## Error Categories

### Base Error Constants
Standardized error constants for common scenarios:
- **Validation Errors**: `ErrInvalidInput`, `ErrMissingRequired`, `ErrInvalidFormat`
- **Permission Errors**: `ErrPermissionDenied`, `ErrUnauthorized`, `ErrUserDenied`
- **Session Errors**: `ErrSessionNotFound`, `ErrSessionBusy`, `ErrRequestCancelled`
- **Tool Errors**: `ErrToolNotFound`, `ErrToolCallFailed`, `ErrCommandFailed`
- **File System Errors**: `ErrFileNotFound`, `ErrFileAccessDenied`, `ErrInvalidPath`
- **Configuration Errors**: `ErrInvalidConfig`, `ErrMissingConfig`, `ErrProviderFailed`

### Structured Error Types
Rich error types with additional context:
- **ValidationError**: Field validation with field name, value, and message
- **PermissionError**: Permission context with resource, action, and session
- **ToolError**: Tool execution context with tool name, action, and cause
- **StateError**: State transition context with current, target, and operation
- **ConfigError**: Configuration issues with key, value, and expected format

## Helper Functions

### Error Creation Helpers
```go
// Validation errors
errors.Validation(field string, value any, message string) error
errors.Permission(resource, action, session string) error
errors.Tool(toolName, action string, cause error) error
errors.State(current, target, operation, description string) error
errors.Config(key string, value any, expected, description string) error
errors.ConfigSimple(description string) error

// File operation helpers
errors.File(operation, path string, cause error) error
errors.FileSimple(operation string, cause error) error

// Session and provider helpers
errors.Session(description string) error
errors.Provider(description, providerID string) error
errors.ProviderWithCause(description, providerID string, cause error) error
```

### Error Wrapping Helpers
```go
errors.Wrap(err error, message string) error
errors.WithContext(err error, operation, context string) error
errors.WithField(err error, field string, value any) error
```

## Error Categorization Functions
```go
// Type checking functions
errors.IsValidationError(err error) bool
errors.IsPermissionError(err error) bool
errors.IsToolError(err error) bool
errors.IsStateError(err error) bool
errors.IsConfigError(err error) bool

// Error behavior analysis
errors.IsRetryable(err error) bool      // Can this error be retried?
errors.IsPermanent(err error) bool      // Is this a permanent failure?
```

## Migration Summary

### Successfully Migrated Files
1. **internal/agent/tools/edit.go** - 18 error patterns centralized
2. **internal/agent/tools/multiedit.go** - 15 error patterns centralized
3. **internal/config/load.go** - 14 error patterns centralized
4. **internal/config/config.go** - 12 error patterns centralized

**Total: 59+ error patterns centralized**

### Migration Patterns Applied
- `fmt.Errorf("failed to write file: %w", err)` → `errors.File("write", filePath, err)`
- `fmt.Errorf("session ID is required...")` → `errors.Session("session ID is required")`
- `fmt.Errorf("provider %s not found", id)` → `errors.Provider("not found", id)`
- `fmt.Errorf("validation failed...")` → `errors.Validation("field", value, "message")`
- `fmt.Errorf("configuration error: %w", err)` → `errors.Wrap(err, "configuration error")`

### Re-Export Pattern
For backward compatibility, existing error files have been updated to re-export centralized errors:

```go
// internal/message/errors.go
var ErrToolCallIDEmpty = errors.ErrToolCallIDEmpty

// internal/agent/errors.go  
var (
    ErrRequestCancelled = errors.ErrRequestCancelled
    ErrSessionBusy      = errors.ErrSessionBusy
    ErrEmptyPrompt      = errors.ErrEmptyInput  // Mapped to centralized equivalent
    ErrSessionMissing   = errors.ErrSessionMissing
)

// internal/enum/errors.go
var (
    ErrToolCallStateUnknown   = errors.ErrToolCallStateUnknown
    ErrAnimationStateUnknown  = errors.ErrAnimationStateUnknown
    ErrToolResultStateUnknown = errors.ErrToolResultStateUnknown
)

// internal/permission/permission.go
var ErrorPermissionDenied = errors.ErrUserDenied
```

## Benefits Achieved

1. **Consistency**: All error messages follow standardized patterns
2. **Maintainability**: Single source of truth for error definitions
3. **Type Safety**: Structured error types with proper field validation
4. **Context Preservation**: Error wrapping maintains full context chain
5. **Categorization**: Easy error type checking for different handling strategies
6. **Internationalization Ready**: Centralized messages make i18n implementation easier
7. **Testing**: Easier to test error scenarios and simulate conditions
8. **Monitoring**: Better error tracking and analytics capabilities

## Usage Examples

### Basic Error Creation
```go
// File operation error
err := errors.File("write", "/path/to/file", osErr)

// Validation error
err := errors.Validation("email", email, "invalid email format")

// Permission error
err := errors.Permission("read", "sensitive-data", sessionID)
```

### Error Handling
```go
if errors.IsPermissionError(err) {
    // Handle permission errors
} else if errors.IsRetryable(err) {
    // Retry the operation
} else if errors.IsPermanent(err) {
    // Show permanent failure message
}
```

### Error Wrapping
```go
err := someOperation()
if err != nil {
    return errors.Wrap(err, "failed to process request")
}
```

## Future Enhancements

1. **Error Codes**: Add error codes for programmatic error handling
2. **Localization**: Support for multiple languages via error code mapping
3. **Metrics Integration**: Automatic error tracking and reporting
4. **Structured Logging**: Integration with structured logging systems
5. **Error Recovery**: Built-in recovery strategies for retryable errors

## Guidelines for Developers

1. **Always prefer centralized helpers** over `fmt.Errorf()` or `errors.New()`
2. **Use appropriate structured types** for complex error scenarios
3. **Wrap errors properly** to maintain context chains
4. **Check error types** using the provided helper functions
5. **Document new error patterns** when adding to the centralized system

This centralized error system provides a solid foundation for consistent, maintainable error handling throughout the Crush application.