// Code generated manually. NOT by sqlc.
// Dispatch queries - manually implemented until sqlc generation is available.

package db

import (
	"context"
	"database/sql"
)

// CreateDispatchMessageParams contains parameters for CreateDispatchMessage.
type CreateDispatchMessageParams struct {
	ID              string
	FromAgent       string
	ToAgent         string
	SessionID       sql.NullString
	ParentMessageID sql.NullString
	Task            string
	Context         string
	Priority        int64
}

// ListDispatchMessagesParams contains parameters for ListDispatchMessages.
type ListDispatchMessagesParams struct {
	FromAgent string
	ToAgent   string
	Status    string
}

// PollDispatchMessagesParams contains parameters for PollDispatchMessages.
type PollDispatchMessagesParams struct {
	ToAgent string
	Limit   int64
}

// CompleteDispatchMessageParams contains parameters for CompleteDispatchMessage.
type CompleteDispatchMessageParams struct {
	ID     string
	Result sql.NullString
}

// FailDispatchMessageParams contains parameters for FailDispatchMessage.
type FailDispatchMessageParams struct {
	ID    string
	Error sql.NullString
}

// CreateAgentParams contains parameters for CreateAgent.
type CreateAgentParams struct {
	Name              string
	Description       sql.NullString
	Capabilities      string
	SystemPrompt      sql.NullString
	CliCommand        sql.NullString
	ModelRequirements string
}

// UpdateAgentParams contains parameters for UpdateAgent.
type UpdateAgentParams struct {
	Name              string
	Description       sql.NullString
	Capabilities      string
	SystemPrompt      sql.NullString
	CliCommand        sql.NullString
	ModelRequirements string
}

// SetAgentEnabledParams contains parameters for SetAgentEnabled.
type SetAgentEnabledParams struct {
	Name    string
	Enabled int64
}

// CreateDispatchMessage creates a new dispatch message.
func (q *Queries) CreateDispatchMessage(ctx context.Context, arg CreateDispatchMessageParams) (DispatchMessage, error) {
	const query = `
		INSERT INTO dispatch_messages (
			id, from_agent, to_agent, session_id, parent_message_id, task, context, status, priority, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, strftime('%s', 'now'), strftime('%s', 'now'))
		RETURNING id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
	`
	var row DispatchMessage
	err := q.db.QueryRowContext(ctx, query,
		arg.ID, arg.FromAgent, arg.ToAgent, arg.SessionID, arg.ParentMessageID, arg.Task, arg.Context, arg.Priority,
	).Scan(
		&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
	)
	return row, err
}

// GetDispatchMessage retrieves a dispatch message by ID.
func (q *Queries) GetDispatchMessage(ctx context.Context, id string) (DispatchMessage, error) {
	const query = `SELECT id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at FROM dispatch_messages WHERE id = ? LIMIT 1`
	var row DispatchMessage
	err := q.db.QueryRowContext(ctx, query, id).Scan(
		&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
	)
	return row, err
}

// ListDispatchMessages lists dispatch messages with optional filters.
func (q *Queries) ListDispatchMessages(ctx context.Context, arg ListDispatchMessagesParams) ([]DispatchMessage, error) {
	const query = `
		SELECT id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
		FROM dispatch_messages
		WHERE (? = '' OR from_agent = ?)
		  AND (? = '' OR to_agent = ?)
		  AND (? = '' OR status = ?)
		ORDER BY priority DESC, created_at ASC
	`
	rows, err := q.db.QueryContext(ctx, query,
		arg.FromAgent, arg.FromAgent,
		arg.ToAgent, arg.ToAgent,
		arg.Status, arg.Status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DispatchMessage
	for rows.Next() {
		var row DispatchMessage
		if err := rows.Scan(
			&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// PollDispatchMessages retrieves pending messages for an agent.
func (q *Queries) PollDispatchMessages(ctx context.Context, arg PollDispatchMessagesParams) ([]DispatchMessage, error) {
	const query = `
		SELECT id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
		FROM dispatch_messages
		WHERE to_agent = ? AND status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT ?
	`
	rows, err := q.db.QueryContext(ctx, query, arg.ToAgent, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DispatchMessage
	for rows.Next() {
		var row DispatchMessage
		if err := rows.Scan(
			&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// ClaimDispatchMessage claims a pending message.
func (q *Queries) ClaimDispatchMessage(ctx context.Context, id string) (DispatchMessage, error) {
	const query = `
		UPDATE dispatch_messages SET status = 'in_progress', updated_at = strftime('%s', 'now')
		WHERE id = ? AND status = 'pending'
		RETURNING id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
	`
	var row DispatchMessage
	err := q.db.QueryRowContext(ctx, query, id).Scan(
		&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
	)
	return row, err
}

// CompleteDispatchMessage marks a message as completed.
func (q *Queries) CompleteDispatchMessage(ctx context.Context, arg CompleteDispatchMessageParams) (DispatchMessage, error) {
	const query = `
		UPDATE dispatch_messages
		SET status = 'completed', result = ?, completed_at = strftime('%s', 'now'), updated_at = strftime('%s', 'now')
		WHERE id = ?
		RETURNING id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
	`
	var row DispatchMessage
	err := q.db.QueryRowContext(ctx, query, arg.Result, arg.ID).Scan(
		&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
	)
	return row, err
}

// FailDispatchMessage marks a message as failed.
func (q *Queries) FailDispatchMessage(ctx context.Context, arg FailDispatchMessageParams) (DispatchMessage, error) {
	const query = `
		UPDATE dispatch_messages
		SET status = 'failed', error = ?, completed_at = strftime('%s', 'now'), updated_at = strftime('%s', 'now')
		WHERE id = ?
		RETURNING id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
	`
	var row DispatchMessage
	err := q.db.QueryRowContext(ctx, query, arg.Error, arg.ID).Scan(
		&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
	)
	return row, err
}

// DeleteDispatchMessage deletes a dispatch message.
func (q *Queries) DeleteDispatchMessage(ctx context.Context, id string) error {
	const query = `DELETE FROM dispatch_messages WHERE id = ?`
	_, err := q.db.ExecContext(ctx, query, id)
	return err
}

// ListPendingDispatchMessages lists all pending messages.
func (q *Queries) ListPendingDispatchMessages(ctx context.Context) ([]DispatchMessage, error) {
	const query = `
		SELECT id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
		FROM dispatch_messages WHERE status = 'pending' ORDER BY created_at ASC
	`
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DispatchMessage
	for rows.Next() {
		var row DispatchMessage
		if err := rows.Scan(
			&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// ListStaleDispatchMessages lists messages in-progress for too long.
func (q *Queries) ListStaleDispatchMessages(ctx context.Context, cutoff int64) ([]DispatchMessage, error) {
	const query = `
		SELECT id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
		FROM dispatch_messages WHERE status = 'in_progress' AND updated_at < ? ORDER BY updated_at ASC
	`
	rows, err := q.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DispatchMessage
	for rows.Next() {
		var row DispatchMessage
		if err := rows.Scan(
			&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// ResetDispatchMessage resets a message to pending.
func (q *Queries) ResetDispatchMessage(ctx context.Context, id string) (DispatchMessage, error) {
	const query = `
		UPDATE dispatch_messages SET status = 'pending', updated_at = strftime('%s', 'now')
		WHERE id = ?
		RETURNING id, from_agent, to_agent, session_id, parent_message_id, task, context, status, result, error, priority, created_at, updated_at, completed_at
	`
	var row DispatchMessage
	err := q.db.QueryRowContext(ctx, query, id).Scan(
		&row.Id, &row.FromAgent, &row.ToAgent, &row.SessionID, &row.ParentMessageID, &row.Task, &row.Context, &row.Status, &row.Result, &row.Error, &row.Priority, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt,
	)
	return row, err
}

// CreateAgent creates a new agent.
func (q *Queries) CreateAgent(ctx context.Context, arg CreateAgentParams) (Agent, error) {
	const query = `
		INSERT INTO agents (name, description, capabilities, system_prompt, cli_command, model_requirements, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now'))
		RETURNING name, description, capabilities, system_prompt, cli_command, model_requirements, enabled, created_at, updated_at
	`
	var row Agent
	err := q.db.QueryRowContext(ctx, query,
		arg.Name, arg.Description, arg.Capabilities, arg.SystemPrompt, arg.CliCommand, arg.ModelRequirements,
	).Scan(
		&row.Name, &row.Description, &row.Capabilities, &row.SystemPrompt, &row.CliCommand, &row.ModelRequirements, &row.Enabled, &row.CreatedAt, &row.UpdatedAt,
	)
	return row, err
}

// GetAgent retrieves an agent by name.
func (q *Queries) GetAgent(ctx context.Context, name string) (Agent, error) {
	const query = `SELECT name, description, capabilities, system_prompt, cli_command, model_requirements, enabled, created_at, updated_at FROM agents WHERE name = ? LIMIT 1`
	var row Agent
	err := q.db.QueryRowContext(ctx, query, name).Scan(
		&row.Name, &row.Description, &row.Capabilities, &row.SystemPrompt, &row.CliCommand, &row.ModelRequirements, &row.Enabled, &row.CreatedAt, &row.UpdatedAt,
	)
	return row, err
}

// ListAgents lists enabled agents.
func (q *Queries) ListAgents(ctx context.Context) ([]Agent, error) {
	const query = `SELECT name, description, capabilities, system_prompt, cli_command, model_requirements, enabled, created_at, updated_at FROM agents WHERE enabled = 1 ORDER BY name`
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Agent
	for rows.Next() {
		var row Agent
		if err := rows.Scan(
			&row.Name, &row.Description, &row.Capabilities, &row.SystemPrompt, &row.CliCommand, &row.ModelRequirements, &row.Enabled, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// ListAllAgents lists all agents including disabled.
func (q *Queries) ListAllAgents(ctx context.Context) ([]Agent, error) {
	const query = `SELECT name, description, capabilities, system_prompt, cli_command, model_requirements, enabled, created_at, updated_at FROM agents ORDER BY name`
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Agent
	for rows.Next() {
		var row Agent
		if err := rows.Scan(
			&row.Name, &row.Description, &row.Capabilities, &row.SystemPrompt, &row.CliCommand, &row.ModelRequirements, &row.Enabled, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// UpdateAgent updates an agent.
func (q *Queries) UpdateAgent(ctx context.Context, arg UpdateAgentParams) (Agent, error) {
	const query = `
		UPDATE agents
		SET description = ?, capabilities = ?, system_prompt = ?, cli_command = ?, model_requirements = ?, updated_at = strftime('%s', 'now')
		WHERE name = ?
		RETURNING name, description, capabilities, system_prompt, cli_command, model_requirements, enabled, created_at, updated_at
	`
	var row Agent
	err := q.db.QueryRowContext(ctx, query,
		arg.Description, arg.Capabilities, arg.SystemPrompt, arg.CliCommand, arg.ModelRequirements, arg.Name,
	).Scan(
		&row.Name, &row.Description, &row.Capabilities, &row.SystemPrompt, &row.CliCommand, &row.ModelRequirements, &row.Enabled, &row.CreatedAt, &row.UpdatedAt,
	)
	return row, err
}

// SetAgentEnabled enables or disables an agent.
func (q *Queries) SetAgentEnabled(ctx context.Context, arg SetAgentEnabledParams) error {
	const query = `UPDATE agents SET enabled = ?, updated_at = strftime('%s', 'now') WHERE name = ?`
	_, err := q.db.ExecContext(ctx, query, arg.Enabled, arg.Name)
	return err
}

// DeleteAgent deletes an agent.
func (q *Queries) DeleteAgent(ctx context.Context, name string) error {
	const query = `DELETE FROM agents WHERE name = ?`
	_, err := q.db.ExecContext(ctx, query, name)
	return err
}
