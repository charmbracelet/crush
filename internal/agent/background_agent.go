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
	AgentID         string
	AgentName       string
	AgentType       string
	Description     string
	ChildSessionID  string
	ParentSessionID string
	Status          string
	Content         string
	Summary         string
	FilesTouched    []string
	Artifacts       []string
}

type backgroundAgentCommand struct {
	Prompt         string
	SessionID      string
	AgentMessageID string
	ToolCallID     string
}

type backgroundAgentRunResult struct {
	ChildSessionID string
	Status         backgroundAgentStatus
	Content        string
}

type backgroundAgentRunner func(context.Context, backgroundAgentCommand) backgroundAgentRunResult

// backgroundAgentEntry tracks a single background agent's execution state.
type backgroundAgentEntry struct {
	AgentID         string                `json:"agent_id"`
	AgentName       string                `json:"agent_name,omitempty"` // User-friendly name for addressing
	AgentType       string                `json:"agent_type,omitempty"` // Type: general, explore, etc.
	Description     string                `json:"description"`
	ChildSessionID  string                `json:"child_session_id"`
	ParentSessionID string                `json:"parent_session_id,omitempty"` // Session that spawned this agent
	Status          backgroundAgentStatus `json:"status"`
	CreatedAt       int64                 `json:"created_at"`
	CompletedAt     int64                 `json:"completed_at,omitempty"`
	Content         string                `json:"content,omitempty"`

	// Structured artifacts from agent execution
	Summary      string   `json:"summary,omitempty"`
	FilesTouched []string `json:"files_touched,omitempty"`
	Artifacts    []string `json:"artifacts,omitempty"`

	commands chan backgroundAgentCommand
	runner   backgroundAgentRunner
}

// ToInfo converts the entry to a public-safe struct.
func (e *backgroundAgentEntry) ToInfo() BackgroundAgentInfo {
	return BackgroundAgentInfo{
		AgentID:         e.AgentID,
		AgentName:       e.AgentName,
		AgentType:       e.AgentType,
		Description:     e.Description,
		ChildSessionID:  e.ChildSessionID,
		ParentSessionID: e.ParentSessionID,
		Status:          string(e.Status),
		Content:         e.Content,
		Summary:         e.Summary,
		FilesTouched:    e.FilesTouched,
		Artifacts:       e.Artifacts,
	}
}

// backgroundAgentRegistry manages the lifecycle of asynchronously running agents.
// It is safe for concurrent use.
type backgroundAgentRegistry struct {
	mu       sync.RWMutex
	agents   map[string]*backgroundAgentEntry
	nameToID map[string]string // Agent name to ID mapping for addressing
	broker   *pubsub.Broker[backgroundAgentEntry]
}

func newBackgroundAgentRegistry() *backgroundAgentRegistry {
	return &backgroundAgentRegistry{
		agents:   make(map[string]*backgroundAgentEntry),
		nameToID: make(map[string]string),
		broker:   pubsub.NewBroker[backgroundAgentEntry](),
	}
}

// Register creates a new background agent entry and returns its ID.
// Agents created via Register are not resumable unless a runner is attached.
func (r *backgroundAgentRegistry) Register(description string) string {
	return r.RegisterNamed("", "", description, nil)
}

// RegisterNamed creates a new background agent with a name for addressing.
func (r *backgroundAgentRegistry) RegisterNamed(name, agentType, description string, runner backgroundAgentRunner) string {
	agentID := fmt.Sprintf("a-%s", generateAgentID())

	// Auto-generate name if not provided
	if name == "" {
		name = fmt.Sprintf("agent-%s", agentID)
	}

	entry := &backgroundAgentEntry{
		AgentID:     agentID,
		AgentName:   name,
		AgentType:   agentType,
		Description: description,
		Status:      backgroundAgentStatusRunning,
		CreatedAt:   time.Now().UnixMilli(),
		runner:      runner,
	}
	if runner != nil {
		entry.commands = make(chan backgroundAgentCommand, 16)
	}

	r.mu.Lock()
	r.agents[agentID] = entry
	r.nameToID[name] = agentID
	r.mu.Unlock()
	r.broker.Publish(pubsub.CreatedEvent, *entry)

	// Start command processor if runner provided
	if runner != nil {
		go r.processQueuedCommands(agentID, entry)
	}

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

// SetParentSession associates a parent session ID with a background agent.
func (r *backgroundAgentRegistry) SetParentSession(agentID, parentSessionID string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.ParentSessionID = parentSessionID
	}
	r.mu.Unlock()
}

// LookupByName finds an agent ID by its name.
func (r *backgroundAgentRegistry) LookupByName(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.nameToID[name]
	return id, ok
}

// ResolveAddress resolves either a name or ID to an agent ID.
func (r *backgroundAgentRegistry) ResolveAddress(nameOrID string) (string, bool) {
	// Try as name first
	if id, ok := r.LookupByName(nameOrID); ok {
		return id, true
	}
	// Try as ID
	r.mu.RLock()
	_, ok := r.agents[nameOrID]
	r.mu.RUnlock()
	if ok {
		return nameOrID, true
	}
	return "", false
}

func (r *backgroundAgentRegistry) markRunning(agentID string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.Status = backgroundAgentStatusRunning
		entry.CompletedAt = 0
		r.broker.Publish(pubsub.UpdatedEvent, *entry)
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

func (r *backgroundAgentRegistry) Enqueue(agentID string, command backgroundAgentCommand) (int, error) {
	r.mu.RLock()
	entry, ok := r.agents[agentID]
	r.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("background agent %q not found", agentID)
	}
	if entry.runner == nil || entry.commands == nil {
		return 0, fmt.Errorf("background agent %q does not accept follow-up prompts", agentID)
	}

	depth := len(entry.commands) + 1
	select {
	case entry.commands <- command:
		return depth, nil
	default:
		return 0, fmt.Errorf("background agent %q queue is full", agentID)
	}
}

// ResumeOrCreate resumes a stopped agent or creates a new one if not found.
// Returns the agent ID and whether it was resumed (vs created).
func (r *backgroundAgentRegistry) ResumeOrCreate(name, agentType, description string, runner backgroundAgentRunner) (agentID string, resumed bool) {
	// Try to find existing agent by name
	if id, ok := r.LookupByName(name); ok {
		r.mu.RLock()
		entry, exists := r.agents[id]
		r.mu.RUnlock()

		if exists && entry.runner != nil {
			// Agent exists and can be resumed
			return id, true
		}
	}

	// Create new agent
	agentID = r.RegisterNamed(name, agentType, description, runner)
	return agentID, false
}

// UpdateArtifacts updates the structured output of an agent.
func (r *backgroundAgentRegistry) UpdateArtifacts(agentID string, summary string, filesTouched, artifacts []string) {
	r.mu.Lock()
	if entry, ok := r.agents[agentID]; ok {
		entry.Summary = summary
		if len(filesTouched) > 0 {
			entry.FilesTouched = filesTouched
		}
		if len(artifacts) > 0 {
			entry.Artifacts = artifacts
		}
	}
	r.mu.Unlock()
}

func (r *backgroundAgentRegistry) processQueuedCommands(agentID string, entry *backgroundAgentEntry) {
	for command := range entry.commands {
		r.markRunning(agentID)
		result := entry.runner(context.Background(), command)
		if result.ChildSessionID != "" {
			r.SetChildSession(agentID, result.ChildSessionID)
		}
		switch result.Status {
		case backgroundAgentStatusFailed:
			r.Fail(agentID, result.Content)
		case backgroundAgentStatusCanceled:
			r.Cancel(agentID, result.Content)
		default:
			r.Complete(agentID, result.Content)
		}
	}
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
