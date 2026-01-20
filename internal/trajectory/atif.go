// Package trajectory provides export functionality for the Harbor ATIF
// (Agent Trajectory Interchange Format) v1.4 specification.
package trajectory

import (
	"encoding/json"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// Trajectory represents the root ATIF document structure.
type Trajectory struct {
	SchemaVersion string        `json:"schema_version"`
	SessionID     string        `json:"session_id"`
	Agent         Agent         `json:"agent"`
	Steps         []Step        `json:"steps"`
	FinalMetrics  *FinalMetrics `json:"final_metrics,omitempty"`
	Extra         any           `json:"extra,omitempty"`
}

// Agent describes the agent that generated the trajectory.
type Agent struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	ModelName string `json:"model_name,omitempty"`
}

// Step represents a single step in the trajectory.
type Step struct {
	StepID           int          `json:"step_id"`
	Timestamp        string       `json:"timestamp"`
	Source           string       `json:"source"` // "user", "agent", or "system"
	Message          string       `json:"message"`
	ReasoningContent string       `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall   `json:"tool_calls,omitempty"`
	Observation      *Observation `json:"observation,omitempty"`
	Metrics          *StepMetrics `json:"metrics,omitempty"`
}

// ToolCall represents a tool invocation by the agent.
type ToolCall struct {
	ToolCallID   string `json:"tool_call_id"`
	FunctionName string `json:"function_name"`
	Arguments    any    `json:"arguments"`
}

// Observation contains the results of tool executions.
type Observation struct {
	Results []ObservationResult `json:"results"`
}

// ObservationResult is a single tool result linked to its call.
type ObservationResult struct {
	SourceCallID string `json:"source_call_id,omitempty"`
	Content      string `json:"content,omitempty"`
}

// StepMetrics contains token usage for a single step.
type StepMetrics struct {
	PromptTokens     int64   `json:"prompt_tokens,omitempty"`
	CompletionTokens int64   `json:"completion_tokens,omitempty"`
	CachedTokens     int64   `json:"cached_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

// FinalMetrics contains aggregate metrics for the entire session.
type FinalMetrics struct {
	TotalPromptTokens     int64   `json:"total_prompt_tokens,omitempty"`
	TotalCompletionTokens int64   `json:"total_completion_tokens,omitempty"`
	TotalCostUSD          float64 `json:"total_cost_usd,omitempty"`
	TotalSteps            int     `json:"total_steps,omitempty"`
}

// ExportSession converts a Crush session and its messages to ATIF format.
func ExportSession(
	sess session.Session,
	messages []message.Message,
	agentName string,
	agentVersion string,
	modelName string,
) (*Trajectory, error) {
	traj := &Trajectory{
		SchemaVersion: "ATIF-v1.4",
		SessionID:     sess.ID,
		Agent: Agent{
			Name:      agentName,
			Version:   agentVersion,
			ModelName: modelName,
		},
		Steps: make([]Step, 0, len(messages)),
	}

	stepID := 1
	var lastAgentStep *Step

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			step := convertUserMessage(msg, stepID)
			traj.Steps = append(traj.Steps, step)
			stepID++
			lastAgentStep = nil

		case message.Assistant:
			step := convertAgentMessage(msg, stepID)
			traj.Steps = append(traj.Steps, step)
			lastAgentStep = &traj.Steps[len(traj.Steps)-1]
			stepID++

		case message.Tool:
			// Attach tool results to the last agent step as observations.
			if lastAgentStep != nil {
				attachToolResults(lastAgentStep, msg)
			}
			// Don't create a separate step for tool results.
		}
	}

	// Add final metrics from session totals.
	if sess.PromptTokens > 0 || sess.CompletionTokens > 0 || sess.Cost > 0 {
		traj.FinalMetrics = &FinalMetrics{
			TotalPromptTokens:     sess.PromptTokens,
			TotalCompletionTokens: sess.CompletionTokens,
			TotalCostUSD:          sess.Cost,
			TotalSteps:            len(traj.Steps),
		}
	}

	return traj, nil
}

// convertUserMessage transforms a user message into an ATIF step.
func convertUserMessage(msg message.Message, stepID int) Step {
	return Step{
		StepID:    stepID,
		Timestamp: time.Unix(msg.CreatedAt, 0).UTC().Format(time.RFC3339),
		Source:    "user",
		Message:   msg.Content().Text,
	}
}

// convertAgentMessage transforms an assistant message into an ATIF step.
func convertAgentMessage(msg message.Message, stepID int) Step {
	step := Step{
		StepID:    stepID,
		Timestamp: time.Unix(msg.CreatedAt, 0).UTC().Format(time.RFC3339),
		Source:    "agent",
		Message:   msg.Content().Text,
	}

	// Include reasoning content if present.
	if reasoning := msg.ReasoningContent(); reasoning.Thinking != "" {
		step.ReasoningContent = reasoning.Thinking
	}

	// Convert tool calls.
	toolCalls := msg.ToolCalls()
	if len(toolCalls) > 0 {
		step.ToolCalls = make([]ToolCall, 0, len(toolCalls))
		for _, tc := range toolCalls {
			atifCall := ToolCall{
				ToolCallID:   tc.ID,
				FunctionName: tc.Name,
			}
			// Try to parse arguments as JSON, fall back to string.
			var args any
			if err := json.Unmarshal([]byte(tc.Input), &args); err != nil {
				args = tc.Input
			}
			atifCall.Arguments = args
			step.ToolCalls = append(step.ToolCalls, atifCall)
		}
	}

	return step
}

// attachToolResults attaches tool results from a tool message to an agent step.
func attachToolResults(step *Step, msg message.Message) {
	results := msg.ToolResults()
	if len(results) == 0 {
		return
	}

	if step.Observation == nil {
		step.Observation = &Observation{
			Results: make([]ObservationResult, 0, len(results)),
		}
	}

	for _, tr := range results {
		step.Observation.Results = append(step.Observation.Results, ObservationResult{
			SourceCallID: tr.ToolCallID,
			Content:      tr.Content,
		})
	}
}
