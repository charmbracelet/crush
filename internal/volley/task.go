package volley

import (
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/provider"
	"github.com/bwl/cliffy/internal/llm/tools"
)

// Task represents a single task to execute in a volley
type Task struct {
	Index  int    // Position in the volley (1-indexed)
	Prompt string // The task prompt
}

// TaskResult represents the result of executing a task
type TaskResult struct {
	Task         Task
	Status       TaskStatus
	Output       string
	Error        error
	Duration     time.Duration
	TokensInput  int64
	TokensOutput int64
	TokensTotal  int64
	Cost         float64
	Retries      int
	WorkerID     int
	Model        string // Model ID used for this task
	ToolMetadata []*tools.ExecutionMetadata // Tool execution metadata
}

// TaskStatus represents the execution status of a task
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusSuccess  TaskStatus = "success"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusCanceled TaskStatus = "canceled"
)

// VolleyOptions configures volley execution
type VolleyOptions struct {
	// Context is the shared context prepended to each task (optional)
	Context string

	// MaxConcurrent is the maximum number of concurrent workers
	MaxConcurrent int

	// MaxRetries is the maximum number of retry attempts per task
	MaxRetries int

	// ShowProgress enables live progress output to stderr
	ShowProgress bool

	// ShowSummary enables summary output after execution
	ShowSummary bool

	// OutputFormat controls output formatting (text or json)
	OutputFormat string

	// FailFast stops execution on first task failure
	FailFast bool

	// Estimate shows cost estimation before running
	Estimate bool

	// SkipConfirmation skips the cost confirmation prompt
	SkipConfirmation bool

	// Verbosity controls output detail level
	Verbosity config.VerbosityLevel

	// ShowThinking enables display of LLM thinking/reasoning
	ShowThinking bool

	// ThinkingFormat controls format of thinking output (text or json)
	ThinkingFormat string

	// ShowStats enables token usage and timing statistics
	ShowStats bool

	// EmitToolTrace enables NDJSON tool trace output to stderr
	EmitToolTrace bool
}

// DefaultVolleyOptions returns sensible defaults
func DefaultVolleyOptions() VolleyOptions {
	return VolleyOptions{
		MaxConcurrent:    3, // Conservative default
		MaxRetries:       3,
		ShowProgress:     true, // Show progress by default
		ShowSummary:      false,
		OutputFormat:     "text",
		FailFast:         false,
		Estimate:         false,
		SkipConfirmation: false,
		Verbosity:        config.VerbosityNormal, // Show tool traces by default
	}
}

// VolleySummary contains aggregate results from a volley execution
type VolleySummary struct {
	TotalTasks       int
	SucceededTasks   int
	FailedTasks      int
	CanceledTasks    int
	Duration         time.Duration
	TotalTokens      int64
	TotalCost        float64
	AvgTokensPerTask int64
	MaxConcurrentUsed int
	TotalRetries     int
}

// Usage represents token usage for a task
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// FromProviderUsage converts provider.TokenUsage to Usage
func FromProviderUsage(pu provider.TokenUsage) Usage {
	return Usage{
		InputTokens:  pu.InputTokens + pu.CacheReadTokens,
		OutputTokens: pu.OutputTokens,
		TotalTokens:  pu.InputTokens + pu.CacheReadTokens + pu.OutputTokens,
	}
}

// ProviderError represents a structured provider error for better reporting
type ProviderError struct {
	Task       Task
	Attempt    int
	Error      error
	ErrorClass ErrorClass
	HTTPStatus int
	Message    string
	IsRetrying bool
}
