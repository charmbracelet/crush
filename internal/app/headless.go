// Package app wires together services, coordinates agents, and manages
// application lifecycle.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// OUTPUT FORMATS
// =============================================================================

// HeadlessFormat represents the output format for headless execution.
type HeadlessFormat string

const (
	// FormatText outputs plain text (default, streamed to stdout).
	FormatText HeadlessFormat = "text"
	// FormatJSON outputs structured JSON (buffered, final result only).
	FormatJSON HeadlessFormat = "json"
	// FormatStreamJSON outputs newline-delimited JSON (NDJSON) for real-time streaming.
	// Each event is output as a separate JSON line, compatible with jq, grep, and Unix pipelines.
	FormatStreamJSON HeadlessFormat = "stream-json"
	// FormatRaw outputs only the model response without any formatting or metadata.
	FormatRaw HeadlessFormat = "raw"
)

// String returns the string representation of the format.
func (f HeadlessFormat) String() string {
	return string(f)
}

// IsStreaming returns true if the format streams output in real-time.
func (f HeadlessFormat) IsStreaming() bool {
	return f == FormatText || f == FormatStreamJSON || f == FormatRaw
}

// IsJSON returns true if the format outputs JSON.
func (f HeadlessFormat) IsJSON() bool {
	return f == FormatJSON || f == FormatStreamJSON
}

// ValidateFormat checks if the format string is valid and returns the parsed format.
func ValidateFormat(format string) (HeadlessFormat, bool) {
	switch format {
	case "text", "":
		return FormatText, true
	case "json":
		return FormatJSON, true
	case "stream-json", "stream_json", "ndjson":
		return FormatStreamJSON, true
	case "raw":
		return FormatRaw, true
	default:
		return "", false
	}
}

// AllFormats returns all valid format strings for documentation.
func AllFormats() []string {
	return []string{"text", "json", "stream-json", "raw"}
}

// =============================================================================
// HEADLESS OPTIONS
// =============================================================================

// HeadlessOptions configures headless execution behavior.
// All fields are optional with sensible defaults.
// This struct is deliberately UI-agnostic - no spinner logic, no printing.
type HeadlessOptions struct {
	// Format specifies the output format: text, json, stream-json, or raw.
	// Default: FormatText
	Format HeadlessFormat

	// ModelID optionally overrides the default model.
	// Empty string uses the configured default.
	ModelID string

	// SessionID optionally specifies a session to use or create.
	// Empty string creates a new ephemeral session.
	SessionID string

	// Timeout specifies the maximum execution duration.
	// Zero means no timeout (run until completion or interrupt).
	Timeout time.Duration

	// NoSkills disables agent skills for this execution.
	NoSkills bool

	// Quiet suppresses the spinner (implied when Format is JSON/stream-json).
	Quiet bool

	// MaxTurns limits the number of agent turns (tool use iterations).
	// Zero means unlimited (default agent behavior).
	// Useful for cost control and preventing runaway executions.
	MaxTurns int

	// AllowedTools specifies tools to auto-approve without permission prompts.
	// Empty means use the config defaults and yolo flag behavior.
	// Example: ["view", "ls", "grep", "edit"]
	AllowedTools []string

	// AppendSystemPrompt adds custom instructions to the agent's system prompt.
	// These are appended to the default system prompt, preserving core capabilities.
	AppendSystemPrompt string

	// SystemPrompt completely replaces the agent's system prompt.
	// Use with caution - this overrides Crush's built-in agent instructions.
	// Cannot be used together with AppendSystemPrompt.
	SystemPrompt string

	// Verbose enables detailed debug output for troubleshooting.
	// Outputs additional information to stderr.
	Verbose bool

	// ContinueSession resumes the most recent session instead of creating a new one.
	ContinueSession bool

	// DisableCache disables prompt caching for this execution.
	DisableCache bool

	// OutputFile writes the final output to a file in addition to stdout.
	// Empty string means stdout only.
	OutputFile string

	// InputFile reads the prompt from a file instead of arguments.
	// Empty string means use arguments or stdin.
	InputFile string
}


// Validate checks the options for internal consistency and returns an error if invalid.
func (o HeadlessOptions) Validate() error {
	if o.MaxTurns < 0 {
		return &ValidationError{Field: "max-turns", Message: "must be >= 0"}
	}
	if o.Timeout < 0 {
		return &ValidationError{Field: "timeout", Message: "must be >= 0"}
	}
	if o.SystemPrompt != "" && o.AppendSystemPrompt != "" {
		return &ValidationError{Field: "system-prompt", Message: "cannot use both --system-prompt and --append-system-prompt"}
	}
	return nil
}

// ShouldShowSpinner returns whether a spinner should be displayed.
func (o HeadlessOptions) ShouldShowSpinner() bool {
	// JSON formats always suppress spinner
	if o.Format.IsJSON() {
		return false
	}
	return !o.Quiet
}

// =============================================================================
// TOKEN USAGE & COST TRACKING
// =============================================================================

// TokenUsage contains comprehensive token consumption metrics.
type TokenUsage struct {
	Input       int64 `json:"input"`                  // Input tokens consumed
	Output      int64 `json:"output"`                 // Output tokens generated
	CacheRead   int64 `json:"cache_read,omitempty"`   // Tokens read from cache
	CacheCreate int64 `json:"cache_create,omitempty"` // Tokens written to cache
	Total       int64 `json:"total,omitempty"`        // Total tokens (input + output)
}

// Cost represents estimated cost in USD (optional, provider-dependent).
type Cost struct {
	Input  float64 `json:"input,omitempty"`  // Cost for input tokens
	Output float64 `json:"output,omitempty"` // Cost for output tokens
	Total  float64 `json:"total,omitempty"`  // Total cost
}

// =============================================================================
// HEADLESS RESULT (JSON OUTPUT SCHEMA)
// =============================================================================

// HeadlessResult contains the structured result for JSON output.
// This schema is stable and suitable for programmatic consumption.
// All fields are optional-safe; missing provider data results in zero values.
type HeadlessResult struct {
	// Metadata
	Version   string `json:"version"`             // Schema version for future compatibility
	Timestamp string `json:"timestamp"`           // ISO 8601 timestamp of completion
	Model     string `json:"model"`               // Model name/ID used
	Provider  string `json:"provider,omitempty"`  // Provider name (e.g., "openai", "anthropic")
	Session   string `json:"session"`             // Session ID
	RequestID string `json:"request_id,omitempty"` // Unique request identifier

	// Input/Output
	Input  string `json:"input"`  // Original prompt
	Output string `json:"output"` // Model response

	// Metrics
	Tokens     TokenUsage `json:"tokens"`               // Token usage breakdown
	Cost       *Cost      `json:"cost,omitempty"`       // Estimated cost (if available)
	DurationMS int64      `json:"duration_ms"`          // Execution time in milliseconds
	Turns      int        `json:"turns,omitempty"`      // Number of agent turns taken
	ToolCalls  int        `json:"tool_calls,omitempty"` // Total tool calls made

	// Status
	Status       string   `json:"status"`                  // "success", "error", "interrupted", "timeout"
	Error        string   `json:"error,omitempty"`         // Error message if status != "success"
	ErrorCode    string   `json:"error_code,omitempty"`    // Machine-readable error code
	Warnings     []string `json:"warnings,omitempty"`      // Non-fatal warnings
	StopReason   string   `json:"stop_reason,omitempty"`   // Why generation stopped
	Truncated    bool     `json:"truncated,omitempty"`     // Whether output was truncated
	SafetyRefuse bool     `json:"safety_refuse,omitempty"` // Whether model refused for safety
}

// ResultSchemaVersion is the current schema version.
const ResultSchemaVersion = "1.0.0"

// NewHeadlessResult creates a new HeadlessResult with defaults.
func NewHeadlessResult() HeadlessResult {
	return HeadlessResult{
		Version:   ResultSchemaVersion,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Status:    "success",
	}
}

// WithError sets the result to error status.
func (r HeadlessResult) WithError(err error) HeadlessResult {
	r.Status = "error"
	r.Error = err.Error()
	if he, ok := err.(*HeadlessError); ok {
		r.ErrorCode = he.Code
	}
	return r
}

// ToJSON serializes the result to pretty-printed JSON bytes.
func (r HeadlessResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToCompactJSON serializes the result to compact JSON (single line).
func (r HeadlessResult) ToCompactJSON() ([]byte, error) {
	return json.Marshal(r)
}

// =============================================================================
// STREAM EVENTS (NDJSON OUTPUT)
// =============================================================================

// StreamEventType defines the type of stream event.
type StreamEventType string

const (
	EventStart      StreamEventType = "start"       // Execution started
	EventText       StreamEventType = "text"        // Text content chunk
	EventToolUse    StreamEventType = "tool_use"    // Tool being invoked
	EventToolResult StreamEventType = "tool_result" // Tool execution result
	EventThinking   StreamEventType = "thinking"    // Model thinking/reasoning
	EventProgress   StreamEventType = "progress"    // Progress update
	EventDone       StreamEventType = "done"        // Execution completed
	EventError      StreamEventType = "error"       // Error occurred
)

// StreamEvent represents a single event in stream-json format (NDJSON).
// Each event is output as a separate line for easy parsing in pipelines.
type StreamEvent struct {
	Type      StreamEventType `json:"type"`                 // Event type
	Timestamp int64           `json:"ts"`                   // Unix timestamp in milliseconds
	Content   string          `json:"content,omitempty"`    // Text content or tool name
	ToolID    string          `json:"tool_id,omitempty"`    // Tool call ID
	ToolName  string          `json:"tool_name,omitempty"`  // Tool name
	ToolInput json.RawMessage `json:"tool_input,omitempty"` // Tool input (JSON)
	Delta     bool            `json:"delta,omitempty"`      // True if this is a partial update
	Error     string          `json:"error,omitempty"`      // Error message
	ErrorCode string          `json:"error_code,omitempty"` // Machine-readable error code
	Tokens    *TokenUsage     `json:"tokens,omitempty"`     // Token usage (in done event)
	Duration  int64           `json:"duration_ms,omitempty"` // Duration (in done event)
}

// NewStreamEvent creates a new StreamEvent with the current timestamp.
func NewStreamEvent(eventType StreamEventType) StreamEvent {
	return StreamEvent{
		Type:      eventType,
		Timestamp: time.Now().UnixMilli(),
	}
}

// ToNDJSON serializes the event to compact JSON for NDJSON streaming.
func (e StreamEvent) ToNDJSON() ([]byte, error) {
	return json.Marshal(e)
}

// =============================================================================
// EXIT CODES
// =============================================================================

// ExitCode defines CLI exit codes for headless execution.
// These follow Unix conventions and are suitable for shell scripting.
type ExitCode int

const (
	// ExitSuccess indicates successful execution.
	ExitSuccess ExitCode = 0
	// ExitRuntimeError indicates a model, network, or runtime error.
	ExitRuntimeError ExitCode = 1
	// ExitInvalidInput indicates invalid flags, arguments, or input.
	ExitInvalidInput ExitCode = 2
	// ExitConfigError indicates configuration or authentication error.
	ExitConfigError ExitCode = 3
	// ExitInterrupted indicates the process was interrupted (SIGINT/Ctrl+C).
	ExitInterrupted ExitCode = 130
)

// String returns a human-readable description of the exit code.
func (e ExitCode) String() string {
	switch e {
	case ExitSuccess:
		return "success"
	case ExitRuntimeError:
		return "runtime error"
	case ExitInvalidInput:
		return "invalid input"
	case ExitConfigError:
		return "configuration error"
	case ExitInterrupted:
		return "interrupted"
	default:
		return fmt.Sprintf("unknown (%d)", e)
	}
}

// =============================================================================
// ERROR TYPES
// =============================================================================

// HeadlessError is a structured error type for headless execution.
type HeadlessError struct {
	Code     string   // Machine-readable error code
	Message  string   // Human-readable message
	ExitCode ExitCode // Suggested exit code
	Cause    error    // Underlying error
}

func (e *HeadlessError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *HeadlessError) Unwrap() error {
	return e.Cause
}

// Common error codes
const (
	ErrCodeInvalidFormat    = "INVALID_FORMAT"
	ErrCodeInvalidInput     = "INVALID_INPUT"
	ErrCodeNoPrompt         = "NO_PROMPT"
	ErrCodeFileNotFound     = "FILE_NOT_FOUND"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeInterrupted      = "INTERRUPTED"
	ErrCodeNoProvider       = "NO_PROVIDER"
	ErrCodeAuthFailed       = "AUTH_FAILED"
	ErrCodeModelError       = "MODEL_ERROR"
	ErrCodeRateLimited      = "RATE_LIMITED"
	ErrCodeMaxTurnsExceeded = "MAX_TURNS_EXCEEDED"
	ErrCodeSessionNotFound  = "SESSION_NOT_FOUND"
	ErrCodeInternal         = "INTERNAL_ERROR"
)

// NewHeadlessError creates a new HeadlessError.
func NewHeadlessError(code string, message string, exitCode ExitCode, cause error) *HeadlessError {
	return &HeadlessError{
		Code:     code,
		Message:  message,
		ExitCode: exitCode,
		Cause:    cause,
	}
}

// ValidationError represents a validation error for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("--%s: %s", e.Field, e.Message)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// IsInterrupted checks if an error represents an interrupted execution.
func IsInterrupted(err error) bool {
	var he *HeadlessError
	if errors.As(err, &he) {
		return he.Code == ErrCodeInterrupted
	}
	return false
}

// IsTimeout checks if an error represents a timeout.
func IsTimeout(err error) bool {
	var he *HeadlessError
	if errors.As(err, &he) {
		return he.Code == ErrCodeTimeout
	}
	return false
}

// GetExitCode extracts the appropriate exit code from an error.
func GetExitCode(err error) ExitCode {
	if err == nil {
		return ExitSuccess
	}
	var he *HeadlessError
	if errors.As(err, &he) {
		return he.ExitCode
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ExitInvalidInput
	}
	return ExitRuntimeError
}
