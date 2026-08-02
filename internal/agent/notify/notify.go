// Package notify defines domain notification types for agent events.
// These types are decoupled from UI concerns so the agent can publish
// events without importing UI packages.
package notify

import (
	"fmt"
	"strings"
	"time"
)

// Type identifies the kind of agent notification.
type Type string

const (
	// TypeAgentFinished indicates the agent has completed its turn.
	TypeAgentFinished Type = "agent_finished"
	// TypeReAuthenticate indicates the agent encountered an
	// authentication error and the user needs to re-authenticate.
	TypeReAuthenticate Type = "re_authenticate"
	// TypeAgentError indicates the agent's turn terminated with an
	// error. The error text is carried in Notification.Message.
	TypeAgentError Type = "error"
	// TypeRetry indicates the agent is backing off before retrying a
	// failed provider request (rate limit, 5xx, network error, etc.).
	TypeRetry Type = "retry"
	// TypeAWSSSOAuth indicates AWS SSO credentials have expired and the
	// coordinator is running the configured refresh command. It opens the
	// AWS SSO dialog; a follow-up with the same type carries the SSO URL
	// once it appears in the command output. AWSSOCommand carries the
	// command being run; AWSSOURL carries the verification URL when known.
	TypeAWSSSOAuth Type = "aws_sso_auth"
	// TypeAWSSSOAuthResult indicates the AWS SSO refresh command has
	// finished. Message carries the error text when it failed, empty on
	// success.
	TypeAWSSSOAuthResult Type = "aws_sso_auth_result"
)

// TypeWorkflowProgress indicates the workflow engine has new live progress.
const TypeWorkflowProgress Type = "workflow_progress"

// WorkflowProgress carries live progress for a running workflow tool.
type WorkflowProgress struct {
	ToolCallID string
	Kind       string // "log" | "agent_start" | "agent_done" | "agent_error"
	Index      int
	Label      string
	Message    string
	Running    int
	Completed  int
	Total      int
}

// Notification represents a domain event published by the agent.
type Notification struct {
	SessionID    string
	SessionTitle string
	Type         Type
	ProviderID   string
	// RunID, when non-empty, is the caller-supplied correlator
	// (proto.AgentMessage.RunID) for the run that produced this
	// notification. It lets observers attribute a TypeAgentError to a
	// specific request rather than to any in-flight run on the
	// session. Empty when no caller set one.
	RunID string
	// Message carries the error text for TypeAgentError, or a short
	// reason for TypeRetry (e.g. provider error title).
	Message string
	// WorkflowProgress carries live progress for a running workflow tool.
	// Only populated when Type is TypeWorkflowProgress.
	WorkflowProgress *WorkflowProgress
	// RetryDelay is how long the agent will wait before the next
	// attempt when Type is TypeRetry.
	RetryDelay time.Duration
	// Attempt is the 1-based retry attempt number for TypeRetry.
	Attempt int
	// MaxRetries is the configured retry budget for TypeRetry.
	MaxRetries int
	// AWSSOCommand carries the shell command for TypeAWSSSOAuth.
	AWSSOCommand string
	// AWSSOURL carries the SSO verification URL for TypeAWSSSOAuth once it
	// appears in the refresh command's output.
	AWSSOURL string
}

// RunComplete is the authoritative end-of-run signal for a session.
// It is published exactly once per top-level agent run (per
// [sessionAgent.Run] invocation that actually executed) after all
// message updates for the turn have been flushed via
// message.Service.FlushAll. Carries the final assistant text and
// message ID so non-interactive clients can reconcile stdout even if
// SSE events arrive out of order or are dropped by the broker. Error
// is non-empty when the run terminated with an error; Cancelled is
// true when the run terminated due to context cancellation. The two
// are mutually exclusive in the success case but may overlap when a
// cancel triggers a downstream error.
//
// RunID identifies the specific request that produced this event.
// It is the value the caller set on `proto.AgentMessage.RunID` (or
// equivalently propagated via agent.WithRunID on the context that
// reaches the coordinator); empty when no caller set one. Filtering
// by RunID lets a client correlate a SendMessage call with its
// terminal event even when the session is busy and other turns are
// finishing on the same session.
type RunComplete struct {
	SessionID string
	RunID     string
	MessageID string
	Text      string
	Error     string
	Cancelled bool
}

// FormatRetryStatus builds the user-facing retry countdown line shown in
// the status bar and on the assistant working spinner while the agent
// backs off before the next provider attempt.
//
// remaining is how long is left until the next try; values under one
// second still render as "1s" so the UI never flashes a zero countdown.
func FormatRetryStatus(n Notification, remaining time.Duration) string {
	if remaining < time.Second {
		remaining = time.Second
	}
	// Round up so "4.1s left" shows as 5s rather than dropping early.
	secs := int((remaining + time.Second - 1) / time.Second)
	left := n.MaxRetries - n.Attempt + 1
	msg := fmt.Sprintf("Retrying in %ds (%d/%d retries left)", secs, left, n.MaxRetries)
	reason := strings.TrimSpace(n.Message)
	if n.ProviderID != "" && reason != "" && !strings.Contains(strings.ToLower(reason), strings.ToLower(n.ProviderID)) {
		reason = n.ProviderID + ": " + reason
	} else if n.ProviderID != "" && reason == "" {
		reason = n.ProviderID
	}
	if reason != "" {
		msg += " - " + reason
	}
	return msg
}
