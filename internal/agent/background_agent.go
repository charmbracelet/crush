package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// backgroundAgentStatus represents the lifecycle state of a background agent.
type backgroundAgentStatus string

const (
	backgroundAgentStatusRunning   backgroundAgentStatus = "running"
	backgroundAgentStatusCompleted backgroundAgentStatus = "completed"
	backgroundAgentStatusFailed    backgroundAgentStatus = "failed"
	backgroundAgentStatusCanceled  backgroundAgentStatus = "canceled"
)

// BackgroundAgentInfo is the public interface for background agent data
// that can be safely passed to other packages.
type BackgroundAgentInfo struct {
	AgentID        string
	Description    string
	ChildSessionID string
	Status         string
	Content        string
}

// backgroundAgentEntry tracks a single background agent's execution state.
type backgroundAgentEntry struct {
	AgentID        string                `json:"agent_id"`
	Description    string                `json:"description"`
	ChildSessionID string                `json:"child_session_id"`
	Status         backgroundAgentStatus `json:"status"`
	CreatedAt      int64                 `json:"created_at"`
	CompletedAt    int64                 `json:"completed_at,omitempty"`
	Content        string                `json:"content,omitempty"`
}

// ToInfo converts the entry to a public-safe struct.
func (e *backgroundAgentEntry) ToInfo() BackgroundAgentInfo {
	return BackgroundAgentInfo{
		AgentID:        e.AgentID,
		Description:    e.Description,
		ChildSessionID: e.ChildSessionID,
		Status:         string(e.Status),
		Content:        e.Content,
	}
}

// backgroundAgentRegistry manages the lifecycle of asynchronously running agents.
// It is safe for concurrent use.
type backgroundAgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]*backgroundAgentEntry
	broker *pubsub.Broker[backgroundAgentEntry]
}

func newBackgroundAgentRegistry() *backgroundAgentRegistry {
	return &backgroundAgentRegistry{
		agents: make(map[string]*backgroundAgentEntry),
		broker: pubsub.NewBroker[backgroundAgentEntry](),
	}
}

// Register creates a new background agent entry and returns its ID.
func (r *backgroundAgentRegistry) Register(description string) string {
	agentID := fmt.Sprintf("a-%s", generateAgentID())
	entry := &backgroundAgentEntry{
		AgentID:     agentID,
		Description: description,
		Status:      backgroundAgentStatusRunning,
		CreatedAt:   time.Now().UnixMilli(),
	}
	r.mu.Lock()
	r.agents[agentID] = entry
	r.mu.Unlock()
	r.broker.Publish(pubsub.CreatedEvent, *entry)
	return agentID
}

// SetChildSession associates a child session ID with a background agent.
func (r *backgroundAgentRegistry) SetChildSession(agentID, childSessionID string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.ChildSessionID = childSessionID
	}
	r.mu.Unlock()
}

// Complete marks a background agent as completed with the given result content.
func (r *backgroundAgentRegistry) Complete(agentID, content string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.Status = backgroundAgentStatusCompleted
		entry.CompletedAt = time.Now().UnixMilli()
		entry.Content = content
		r.broker.Publish(pubsub.UpdatedEvent, *entry)
	}
	r.mu.Unlock()
}

// Fail marks a background agent as failed with an error message.
func (r *backgroundAgentRegistry) Fail(agentID, errMsg string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.Status = backgroundAgentStatusFailed
		entry.CompletedAt = time.Now().UnixMilli()
		entry.Content = errMsg
		r.broker.Publish(pubsub.UpdatedEvent, *entry)
	}
	r.mu.Unlock()
}

// Cancel marks a background agent as canceled.
func (r *backgroundAgentRegistry) Cancel(agentID, reason string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.Status = backgroundAgentStatusCanceled
		entry.CompletedAt = time.Now().UnixMilli()
		entry.Content = reason
		r.broker.Publish(pubsub.UpdatedEvent, *entry)
	}
	r.mu.Unlock()
}

// Get retrieves a background agent entry by ID.
func (r *backgroundAgentRegistry) Get(agentID string) (*backgroundAgentEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.agents[agentID]
	if !ok {
		return nil, false
	}
	copy := *entry
	return &copy, true
}

// List returns all background agent entries.
func (r *backgroundAgentRegistry) List() []backgroundAgentEntry {
	r.mu.RLock()
	entries := make([]backgroundAgentEntry, 0, len(r.agents))
	for _, entry := range r.agents {
		entries = append(entries, *entry)
	}
	r.mu.RUnlock()
	return entries
}

// Subscribe returns a channel that receives updates for background agents.
func (r *backgroundAgentRegistry) Subscribe(ctx context.Context) <-chan pubsub.Event[backgroundAgentEntry] {
	return r.broker.Subscribe(ctx)
}

// LookupFunc is the function type for looking up background agent info.
type LookupFunc func(agentID string) (BackgroundAgentInfo, bool)

// Lookup returns a function that can be passed via context.
func (r *backgroundAgentRegistry) Lookup() LookupFunc {
	return func(agentID string) (BackgroundAgentInfo, bool) {
		entry, ok := r.Get(agentID)
		if !ok {
			return BackgroundAgentInfo{}, false
		}
		return entry.ToInfo(), true
	}
}

// generateAgentID creates a short unique identifier for background agents.
func generateAgentID() string {
	return uuid.New().String()[:8]
}

// backgroundAgentResultNotification is the XML-formatted notification injected
// into the coordinator's context when a background agent completes.
const backgroundAgentResultNotification = `<task-notification>
<task-id>%s</task-id>
<status>%s</status>
<summary>Agent %q %s</summary>
<result>%s</result>
</task-notification>`

// formatBackgroundAgentNotification creates an XML notification for a completed background agent.
func formatBackgroundAgentNotification(entry *backgroundAgentEntry) string {
	status := string(entry.Status)
	action := "completed"
	if entry.Status == backgroundAgentStatusFailed {
		action = "failed"
	} else if entry.Status == backgroundAgentStatusCanceled {
		action = "was canceled"
	}
	content := entry.Content
	if len([]rune(content)) > 2000 {
		content = string([]rune(content)[:2000]) + "\n…[truncated]"
	}
	return fmt.Sprintf(backgroundAgentResultNotification, entry.AgentID, status, entry.Description, action, content)
}

// backgroundAgentSubtaskResult converts a backgroundAgentEntry to a ToolResultSubtaskResult.
func backgroundAgentSubtaskResult(entry *backgroundAgentEntry) message.ToolResultSubtaskResult {
	status := message.ToolResultSubtaskStatusCompleted
	if entry.Status == backgroundAgentStatusFailed {
		status = message.ToolResultSubtaskStatusFailed
	} else if entry.Status == backgroundAgentStatusCanceled {
		status = message.ToolResultSubtaskStatusCanceled
	}
	return message.ToolResultSubtaskResult{
		ChildSessionID:   entry.ChildSessionID,
		ParentToolCallID: entry.AgentID,
		Status:           status,
	}
}

// runBackgroundTask executes a task graph in the background and returns immediately.
func (c *coordinator) runBackgroundTask(ctx context.Context, params taskGraphParams) (fantasy.ToolResponse, error) {
	description := "background task"
	if len(params.Tasks) == 1 && params.Tasks[0].Description != "" {
		description = params.Tasks[0].Description
	}

	agentID := c.backgroundAgents.Register(description)

	// Launch the task in a background goroutine.
	go func() {
		bgCtx := context.Background()
		result, err := c.runTaskGraphDirect(bgCtx, params)
		if err != nil {
			slog.Error("Background agent failed", "agent_id", agentID, "error", err)
			c.backgroundAgents.Fail(agentID, err.Error())
			return
		}
		content := result.Content
		if content == "" {
			content = "Background agent completed with no output."
		}

		// Extract child session ID from metadata before publishing completion event.
		var childSessionID string
		if result.Metadata != "" {
			if sub, ok := message.ParseToolResultSubtaskResult(result.Metadata); ok && sub.ChildSessionID != "" {
				childSessionID = sub.ChildSessionID
				c.backgroundAgents.SetChildSession(agentID, childSessionID)
			}
		}

		_ = childSessionID
		c.backgroundAgents.Complete(agentID, content)
	}()

	return fantasy.NewTextResponse(fmt.Sprintf(
		`Background agent launched.

Agent ID: %s
Description: %s

The agent is now running in the background. To retrieve the result later:
- Use the subtask_result tool with agent_id="%s"

You can continue with other work while the agent runs.`,
		agentID, description, agentID,
	)), nil
}
