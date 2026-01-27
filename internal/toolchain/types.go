// Package toolchain provides types and utilities for summarizing chains of tool calls
// made by the AI agent during a conversation turn.
package toolchain

import "time"

// ToolCall represents a single tool invocation within a chain.
type ToolCall struct {
	// ID is the unique identifier for this tool call.
	ID string `json:"id"`
	// Name is the name of the tool that was called.
	Name string `json:"name"`
	// Input contains the input parameters passed to the tool.
	Input string `json:"input"`
	// Output contains the result returned by the tool.
	Output string `json:"output"`
	// IsError indicates whether the tool call resulted in an error.
	IsError bool `json:"is_error"`
	// StartedAt is when the tool call began execution.
	StartedAt time.Time `json:"started_at"`
	// FinishedAt is when the tool call completed.
	FinishedAt time.Time `json:"finished_at"`
}

// Duration returns the execution time of the tool call.
func (tc *ToolCall) Duration() time.Duration {
	if tc.FinishedAt.IsZero() || tc.StartedAt.IsZero() {
		return 0
	}
	return tc.FinishedAt.Sub(tc.StartedAt)
}

// Chain represents a sequence of tool calls made during a single conversation turn.
type Chain struct {
	// SessionID identifies the session this chain belongs to.
	SessionID string `json:"session_id"`
	// MessageID identifies the assistant message that initiated this chain.
	MessageID string `json:"message_id"`
	// Calls contains the ordered sequence of tool calls in this chain.
	Calls []ToolCall `json:"calls"`
	// StartedAt is when the first tool call in the chain began.
	StartedAt time.Time `json:"started_at"`
	// FinishedAt is when the last tool call in the chain completed.
	FinishedAt time.Time `json:"finished_at"`
}

// Add appends a tool call to the chain.
func (c *Chain) Add(call ToolCall) {
	if len(c.Calls) == 0 {
		c.StartedAt = call.StartedAt
	}
	c.Calls = append(c.Calls, call)
	if !call.FinishedAt.IsZero() {
		c.FinishedAt = call.FinishedAt
	}
}

// Len returns the number of tool calls in the chain.
func (c *Chain) Len() int {
	return len(c.Calls)
}

// Duration returns the total execution time of the chain.
func (c *Chain) Duration() time.Duration {
	if c.FinishedAt.IsZero() || c.StartedAt.IsZero() {
		return 0
	}
	return c.FinishedAt.Sub(c.StartedAt)
}

// IsEmpty returns true if the chain has no tool calls.
func (c *Chain) IsEmpty() bool {
	return len(c.Calls) == 0
}

// Last returns the last tool call in the chain, or nil if empty.
func (c *Chain) Last() *ToolCall {
	if len(c.Calls) == 0 {
		return nil
	}
	return &c.Calls[len(c.Calls)-1]
}

// ToolNames returns a list of unique tool names used in the chain.
func (c *Chain) ToolNames() []string {
	seen := make(map[string]struct{})
	var names []string
	for _, call := range c.Calls {
		if _, ok := seen[call.Name]; !ok {
			seen[call.Name] = struct{}{}
			names = append(names, call.Name)
		}
	}
	return names
}

// HasErrors returns true if any tool call in the chain resulted in an error.
func (c *Chain) HasErrors() bool {
	for _, call := range c.Calls {
		if call.IsError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of tool calls that resulted in errors.
func (c *Chain) ErrorCount() int {
	count := 0
	for _, call := range c.Calls {
		if call.IsError {
			count++
		}
	}
	return count
}

// Summary holds the summarized representation of a tool chain.
type Summary struct {
	// Chain is the original chain being summarized.
	Chain *Chain `json:"chain"`
	// Text is the human-readable summary of the chain.
	Text string `json:"text"`
	// Collapsed indicates whether the chain should be displayed collapsed.
	Collapsed bool `json:"collapsed"`
	// GeneratedAt is when this summary was created.
	GeneratedAt time.Time `json:"generated_at"`
}

// NewSummary creates a new summary for the given chain.
func NewSummary(chain *Chain, text string) *Summary {
	return &Summary{
		Chain:       chain,
		Text:        text,
		Collapsed:   true,
		GeneratedAt: time.Now(),
	}
}
