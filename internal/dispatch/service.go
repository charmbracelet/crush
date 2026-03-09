// Package dispatch provides inter-agent communication through a document-based dispatch system.
// Workers are spawned on demand, read their task from the API, execute, and submit results.
package dispatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// MessageStatus represents the status of a dispatch message.
type MessageStatus string

const (
	StatusPending    MessageStatus = "pending"
	StatusInProgress MessageStatus = "in_progress"
	StatusCompleted  MessageStatus = "completed"
	StatusFailed     MessageStatus = "failed"
)

// Message represents a dispatch message for inter-agent communication.
type Message struct {
	ID              string         `json:"id"`
	FromAgent       string         `json:"from_agent"`
	ToAgent         string         `json:"to_agent"`
	SessionID       string         `json:"session_id,omitempty"`
	ParentMessageID string         `json:"parent_message_id,omitempty"`
	Task            string         `json:"task"`
	Context         map[string]any `json:"context,omitempty"`
	Status          MessageStatus  `json:"status"`
	Result          string         `json:"result,omitempty"`
	Error           string         `json:"error,omitempty"`
	Priority        int            `json:"priority"`
	CreatedAt       int64          `json:"created_at"`
	UpdatedAt       int64          `json:"updated_at"`
	CompletedAt     *int64         `json:"completed_at,omitempty"`
}

// Agent represents a registered agent in the system.
type Agent struct {
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	Capabilities      []string       `json:"capabilities,omitempty"`
	SystemPrompt      string         `json:"system_prompt,omitempty"`
	CLICommand        string         `json:"cli_command,omitempty"`
	ModelRequirements map[string]any `json:"model_requirements,omitempty"`
	Enabled           bool           `json:"enabled"`
	CreatedAt         int64          `json:"created_at"`
	UpdatedAt         int64          `json:"updated_at"`
}

// SendMessageParams contains parameters for sending a dispatch message.
type SendMessageParams struct {
	FromAgent       string         `json:"from_agent"`
	ToAgent         string         `json:"to_agent"`
	SessionID       string         `json:"session_id,omitempty"`
	ParentMessageID string         `json:"parent_message_id,omitempty"`
	Task            string         `json:"task"`
	Context         map[string]any `json:"context,omitempty"`
	Priority        int            `json:"priority,omitempty"`
}

// ListMessagesParams contains parameters for listing dispatch messages.
type ListMessagesParams struct {
	FromAgent string        `json:"from_agent,omitempty"`
	ToAgent   string        `json:"to_agent,omitempty"`
	Status    MessageStatus `json:"status,omitempty"`
}

// CreateAgentParams contains parameters for creating an agent.
type CreateAgentParams struct {
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	Capabilities      []string       `json:"capabilities,omitempty"`
	SystemPrompt      string         `json:"system_prompt,omitempty"`
	CLICommand        string         `json:"cli_command,omitempty"`
	ModelRequirements map[string]any `json:"model_requirements,omitempty"`
}

// DispatchParams contains parameters for dispatching a task to a worker.
type DispatchParams struct {
	Worker    string         `json:"worker"`
	Variables map[string]any `json:"variables,omitempty"`
	Task      string         `json:"task"` // Rendered task content
	SessionID string         `json:"session_id,omitempty"`
}

// Config contains configuration for the dispatch service.
type Config struct {
	APIEndpoint string // Base URL for the dispatch API (e.g., "http://localhost:8080")
}

// Service provides dispatch operations for inter-agent communication.
type Service interface {
	pubsub.Subscriber[Message]

	// Dispatch creates a document and spawns the worker.
	// This is the primary method for delegating work to workers.
	Dispatch(ctx context.Context, params DispatchParams) (Message, error)

	// Message operations
	Send(ctx context.Context, params SendMessageParams) (Message, error)
	Get(ctx context.Context, id string) (Message, error)
	List(ctx context.Context, params ListMessagesParams) ([]Message, error)
	Poll(ctx context.Context, agentName string, limit int) ([]Message, error)
	Claim(ctx context.Context, id string) (Message, error)
	Complete(ctx context.Context, id, result string) (Message, error)
	Fail(ctx context.Context, id, errMsg string) (Message, error)
	Delete(ctx context.Context, id string) error
	Reset(ctx context.Context, id string) (Message, error)

	// Agent registry operations
	CreateAgent(ctx context.Context, params CreateAgentParams) (Agent, error)
	GetAgent(ctx context.Context, name string) (Agent, error)
	ListAgents(ctx context.Context, includeDisabled bool) ([]Agent, error)
	UpdateAgent(ctx context.Context, name string, params CreateAgentParams) (Agent, error)
	SetAgentEnabled(ctx context.Context, name string, enabled bool) error
	DeleteAgent(ctx context.Context, name string) error
}

type service struct {
	*pubsub.Broker[Message]
	db     *sql.DB
	q      *db.Queries
	config Config
}

// NewService creates a new dispatch service.
func NewService(q *db.Queries, conn *sql.DB, cfg ...Config) Service {
	broker := pubsub.NewBroker[Message]()
	var c Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &service{
		Broker: broker,
		db:     conn,
		q:      q,
		config: c,
	}
}

// Dispatch creates a dispatch document and spawns the worker.
// This is the primary method for delegating work to workers.
func (s *service) Dispatch(ctx context.Context, params DispatchParams) (Message, error) {
	// Validate worker exists and is enabled
	agent, err := s.GetAgent(ctx, params.Worker)
	if err != nil {
		return Message{}, fmt.Errorf("worker %q not found: %w", params.Worker, err)
	}
	if !agent.Enabled {
		return Message{}, fmt.Errorf("worker %q is disabled", params.Worker)
	}
	if agent.CLICommand == "" {
		return Message{}, fmt.Errorf("worker %q has no CLI command configured", params.Worker)
	}

	// Create the dispatch document
	msg, err := s.Send(ctx, SendMessageParams{
		FromAgent: "crush",
		ToAgent:   params.Worker,
		SessionID: params.SessionID,
		Task:      params.Task,
		Context:   params.Variables,
	})
	if err != nil {
		return Message{}, fmt.Errorf("creating dispatch document: %w", err)
	}

	// Spawn the worker
	if err := s.spawnWorker(agent, msg.ID); err != nil {
		// Mark dispatch as failed if spawn fails
		_, _ = s.Fail(ctx, msg.ID, fmt.Sprintf("Failed to spawn worker: %v", err))
		return Message{}, fmt.Errorf("spawning worker: %w", err)
	}

	slog.Info("Worker dispatched", "worker", params.Worker, "dispatch_id", msg.ID)
	return msg, nil
}

// spawnWorker executes the worker's CLI command with the dispatch alert.
func (s *service) spawnWorker(agent Agent, dispatchID string) error {
	// Build the alert message
	alertMsg := fmt.Sprintf("API: %s Dispatch: %s", s.config.APIEndpoint, dispatchID)

	// Replace placeholders in CLI command
	cliCmd := strings.ReplaceAll(agent.CLICommand, "{dispatch_id}", dispatchID)
	cliCmd = strings.ReplaceAll(cliCmd, "{alert}", alertMsg)

	// Parse the command into parts
	parts := strings.Fields(cliCmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty CLI command for worker %s", agent.Name)
	}

	// Append the alert message as the final argument
	cmd := exec.Command(parts[0], append(parts[1:], alertMsg)...)

	// Run asynchronously (don't wait for completion)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			slog.Error("Worker execution failed", "worker", agent.Name, "dispatch_id", dispatchID, "error", err, "output", string(output))
			return
		}
		slog.Debug("Worker completed", "worker", agent.Name, "dispatch_id", dispatchID, "output", string(output))
	}()

	return nil
}

// Send creates a new dispatch message.
func (s *service) Send(ctx context.Context, params SendMessageParams) (Message, error) {
	contextJSON, err := marshalContext(params.Context)
	if err != nil {
		return Message{}, fmt.Errorf("marshaling context: %w", err)
	}

	parentMessageID := sql.NullString{}
	if params.ParentMessageID != "" {
		parentMessageID = sql.NullString{String: params.ParentMessageID, Valid: true}
	}

	sessionID := sql.NullString{}
	if params.SessionID != "" {
		sessionID = sql.NullString{String: params.SessionID, Valid: true}
	}

	dbMsg, err := s.q.CreateDispatchMessage(ctx, db.CreateDispatchMessageParams{
		ID:              uuid.New().String(),
		FromAgent:       params.FromAgent,
		ToAgent:         params.ToAgent,
		SessionID:       sessionID,
		ParentMessageID: parentMessageID,
		Task:            params.Task,
		Context:         contextJSON,
		Priority:        int64(params.Priority),
	})
	if err != nil {
		return Message{}, fmt.Errorf("creating dispatch message: %w", err)
	}

	msg := s.messageFromDB(dbMsg)
	s.Publish(pubsub.CreatedEvent, msg)
	slog.Info("Dispatch message sent", "id", msg.ID, "from", msg.FromAgent, "to", msg.ToAgent)
	return msg, nil
}

// Get retrieves a dispatch message by ID.
func (s *service) Get(ctx context.Context, id string) (Message, error) {
	dbMsg, err := s.q.GetDispatchMessage(ctx, id)
	if err != nil {
		return Message{}, fmt.Errorf("getting dispatch message: %w", err)
	}
	return s.messageFromDB(dbMsg), nil
}

// List retrieves dispatch messages matching the given parameters.
func (s *service) List(ctx context.Context, params ListMessagesParams) ([]Message, error) {
	dbMsgs, err := s.q.ListDispatchMessages(ctx, db.ListDispatchMessagesParams{
		FromAgent: params.FromAgent,
		ToAgent:   params.ToAgent,
		Status:    string(params.Status),
	})
	if err != nil {
		return nil, fmt.Errorf("listing dispatch messages: %w", err)
	}

	msgs := make([]Message, len(dbMsgs))
	for i, dbMsg := range dbMsgs {
		msgs[i] = s.messageFromDB(dbMsg)
	}
	return msgs, nil
}

// Poll retrieves pending messages for an agent.
func (s *service) Poll(ctx context.Context, agentName string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 10
	}

	dbMsgs, err := s.q.PollDispatchMessages(ctx, db.PollDispatchMessagesParams{
		ToAgent: agentName,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("polling dispatch messages: %w", err)
	}

	msgs := make([]Message, len(dbMsgs))
	for i, dbMsg := range dbMsgs {
		msgs[i] = s.messageFromDB(dbMsg)
	}
	return msgs, nil
}

// Claim marks a message as in-progress.
func (s *service) Claim(ctx context.Context, id string) (Message, error) {
	dbMsg, err := s.q.ClaimDispatchMessage(ctx, id)
	if err != nil {
		return Message{}, fmt.Errorf("claiming dispatch message: %w", err)
	}

	msg := s.messageFromDB(dbMsg)
	s.Publish(pubsub.UpdatedEvent, msg)
	slog.Info("Dispatch message claimed", "id", msg.ID, "agent", msg.ToAgent)
	return msg, nil
}

// Complete marks a message as completed with a result.
func (s *service) Complete(ctx context.Context, id, result string) (Message, error) {
	resultSQL := sql.NullString{String: result, Valid: result != ""}

	dbMsg, err := s.q.CompleteDispatchMessage(ctx, db.CompleteDispatchMessageParams{
		ID:     id,
		Result: resultSQL,
	})
	if err != nil {
		return Message{}, fmt.Errorf("completing dispatch message: %w", err)
	}

	msg := s.messageFromDB(dbMsg)
	s.Publish(pubsub.UpdatedEvent, msg)
	slog.Info("Dispatch message completed", "id", msg.ID, "agent", msg.ToAgent)
	return msg, nil
}

// Fail marks a message as failed with an error message.
func (s *service) Fail(ctx context.Context, id, errMsg string) (Message, error) {
	errorSQL := sql.NullString{String: errMsg, Valid: errMsg != ""}

	dbMsg, err := s.q.FailDispatchMessage(ctx, db.FailDispatchMessageParams{
		ID:    id,
		Error: errorSQL,
	})
	if err != nil {
		return Message{}, fmt.Errorf("failing dispatch message: %w", err)
	}

	msg := s.messageFromDB(dbMsg)
	s.Publish(pubsub.UpdatedEvent, msg)
	slog.Info("Dispatch message failed", "id", msg.ID, "agent", msg.ToAgent, "error", errMsg)
	return msg, nil
}

// Delete removes a dispatch message.
func (s *service) Delete(ctx context.Context, id string) error {
	msg, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := s.q.DeleteDispatchMessage(ctx, id); err != nil {
		return fmt.Errorf("deleting dispatch message: %w", err)
	}

	s.Publish(pubsub.DeletedEvent, msg)
	return nil
}

// Reset changes a message status back to pending.
func (s *service) Reset(ctx context.Context, id string) (Message, error) {
	dbMsg, err := s.q.ResetDispatchMessage(ctx, id)
	if err != nil {
		return Message{}, fmt.Errorf("resetting dispatch message: %w", err)
	}

	msg := s.messageFromDB(dbMsg)
	s.Publish(pubsub.UpdatedEvent, msg)
	slog.Info("Dispatch message reset", "id", msg.ID)
	return msg, nil
}

// CreateAgent registers a new agent.
func (s *service) CreateAgent(ctx context.Context, params CreateAgentParams) (Agent, error) {
	capabilitiesJSON, err := marshalCapabilities(params.Capabilities)
	if err != nil {
		return Agent{}, fmt.Errorf("marshaling capabilities: %w", err)
	}

	requirementsJSON, err := marshalModelRequirements(params.ModelRequirements)
	if err != nil {
		return Agent{}, fmt.Errorf("marshaling model requirements: %w", err)
	}

	dbAgent, err := s.q.CreateAgent(ctx, db.CreateAgentParams{
		Name:              params.Name,
		Description:       sql.NullString{String: params.Description, Valid: params.Description != ""},
		Capabilities:      capabilitiesJSON,
		SystemPrompt:      sql.NullString{String: params.SystemPrompt, Valid: params.SystemPrompt != ""},
		CliCommand:        sql.NullString{String: params.CLICommand, Valid: params.CLICommand != ""},
		ModelRequirements: requirementsJSON,
	})
	if err != nil {
		return Agent{}, fmt.Errorf("creating agent: %w", err)
	}

	slog.Info("Agent created", "name", params.Name)
	return s.agentFromDB(dbAgent), nil
}

// GetAgent retrieves an agent by name.
func (s *service) GetAgent(ctx context.Context, name string) (Agent, error) {
	dbAgent, err := s.q.GetAgent(ctx, name)
	if err != nil {
		return Agent{}, fmt.Errorf("getting agent: %w", err)
	}
	return s.agentFromDB(dbAgent), nil
}

// ListAgents retrieves all registered agents.
func (s *service) ListAgents(ctx context.Context, includeDisabled bool) ([]Agent, error) {
	var dbAgents []db.Agent
	var err error

	if includeDisabled {
		dbAgents, err = s.q.ListAllAgents(ctx)
	} else {
		dbAgents, err = s.q.ListAgents(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}

	agents := make([]Agent, len(dbAgents))
	for i, dbAgent := range dbAgents {
		agents[i] = s.agentFromDB(dbAgent)
	}
	return agents, nil
}

// UpdateAgent updates an existing agent.
func (s *service) UpdateAgent(ctx context.Context, name string, params CreateAgentParams) (Agent, error) {
	capabilitiesJSON, err := marshalCapabilities(params.Capabilities)
	if err != nil {
		return Agent{}, fmt.Errorf("marshaling capabilities: %w", err)
	}

	requirementsJSON, err := marshalModelRequirements(params.ModelRequirements)
	if err != nil {
		return Agent{}, fmt.Errorf("marshaling model requirements: %w", err)
	}

	dbAgent, err := s.q.UpdateAgent(ctx, db.UpdateAgentParams{
		Name:              name,
		Description:       sql.NullString{String: params.Description, Valid: params.Description != ""},
		Capabilities:      capabilitiesJSON,
		SystemPrompt:      sql.NullString{String: params.SystemPrompt, Valid: params.SystemPrompt != ""},
		CliCommand:        sql.NullString{String: params.CLICommand, Valid: params.CLICommand != ""},
		ModelRequirements: requirementsJSON,
	})
	if err != nil {
		return Agent{}, fmt.Errorf("updating agent: %w", err)
	}

	slog.Info("Agent updated", "name", name)
	return s.agentFromDB(dbAgent), nil
}

// SetAgentEnabled enables or disables an agent.
func (s *service) SetAgentEnabled(ctx context.Context, name string, enabled bool) error {
	enabledInt := int64(0)
	if enabled {
		enabledInt = 1
	}

	if err := s.q.SetAgentEnabled(ctx, db.SetAgentEnabledParams{
		Name:    name,
		Enabled: enabledInt,
	}); err != nil {
		return fmt.Errorf("setting agent enabled: %w", err)
	}

	slog.Info("Agent enabled status changed", "name", name, "enabled", enabled)
	return nil
}

// DeleteAgent removes an agent from the registry.
func (s *service) DeleteAgent(ctx context.Context, name string) error {
	if err := s.q.DeleteAgent(ctx, name); err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}

	slog.Info("Agent deleted", "name", name)
	return nil
}

func (s *service) messageFromDB(dbMsg db.DispatchMessage) Message {
	context, _ := unmarshalContext(dbMsg.Context)
	var completedAt *int64
	if dbMsg.CompletedAt.Valid {
		completedAt = &dbMsg.CompletedAt.Int64
	}

	return Message{
		ID:              dbMsg.Id,
		FromAgent:       dbMsg.FromAgent,
		ToAgent:         dbMsg.ToAgent,
		SessionID:       dbMsg.SessionID.String,
		ParentMessageID: dbMsg.ParentMessageID.String,
		Task:            dbMsg.Task,
		Context:         context,
		Status:          MessageStatus(dbMsg.Status),
		Result:          dbMsg.Result.String,
		Error:           dbMsg.Error.String,
		Priority:        int(dbMsg.Priority),
		CreatedAt:       dbMsg.CreatedAt,
		UpdatedAt:       dbMsg.UpdatedAt,
		CompletedAt:     completedAt,
	}
}

func (s *service) agentFromDB(dbAgent db.Agent) Agent {
	capabilities, _ := unmarshalCapabilities(dbAgent.Capabilities)
	requirements, _ := unmarshalModelRequirements(dbAgent.ModelRequirements)

	return Agent{
		Name:              dbAgent.Name,
		Description:       dbAgent.Description.String,
		Capabilities:      capabilities,
		SystemPrompt:      dbAgent.SystemPrompt.String,
		CLICommand:        dbAgent.CliCommand.String,
		ModelRequirements: requirements,
		Enabled:           dbAgent.Enabled == 1,
		CreatedAt:         dbAgent.CreatedAt,
		UpdatedAt:         dbAgent.UpdatedAt,
	}
}

func marshalContext(ctx map[string]any) (string, error) {
	if ctx == nil {
		return "{}", nil
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalContext(data string) (map[string]any, error) {
	if data == "" || data == "{}" {
		return nil, nil
	}
	var ctx map[string]any
	if err := json.Unmarshal([]byte(data), &ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

func marshalCapabilities(caps []string) (string, error) {
	if len(caps) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(caps)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalCapabilities(data string) ([]string, error) {
	if data == "" || data == "[]" {
		return nil, nil
	}
	var caps []string
	if err := json.Unmarshal([]byte(data), &caps); err != nil {
		return nil, err
	}
	return caps, nil
}

func marshalModelRequirements(reqs map[string]any) (string, error) {
	if reqs == nil {
		return "{}", nil
	}
	data, err := json.Marshal(reqs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalModelRequirements(data string) (map[string]any, error) {
	if data == "" || data == "{}" {
		return nil, nil
	}
	var reqs map[string]any
	if err := json.Unmarshal([]byte(data), &reqs); err != nil {
		return nil, err
	}
	return reqs, nil
}

// GetStaleMessages retrieves messages that have been in-progress for too long.
func (s *service) GetStaleMessages(ctx context.Context, timeout time.Duration) ([]Message, error) {
	cutoff := time.Now().Add(-timeout).Unix()
	dbMsgs, err := s.q.ListStaleDispatchMessages(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("getting stale messages: %w", err)
	}

	msgs := make([]Message, len(dbMsgs))
	for i, dbMsg := range dbMsgs {
		msgs[i] = s.messageFromDB(dbMsg)
	}
	return msgs, nil
}
