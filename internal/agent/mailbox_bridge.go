package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

type taskGraphMailboxBridge struct {
	service   mailbox.Service
	session   session.Service
	mailbox   string
	sessionID string

	mu    sync.Mutex
	state map[string]*taskGraphNodeState
}

type taskGraphNodeState struct {
	TaskID      string
	Description string
	Status      message.ToolResultSubtaskStatus
	Content     string
	Messages    []string
	ToolUses    int
	LastTool    string
}

func newTaskGraphMailboxBridge(service mailbox.Service, sessions session.Service, sessionID, mailboxID string, tasks []taskGraphTask) (*taskGraphMailboxBridge, error) {
	if service == nil {
		return nil, fmt.Errorf("mailbox service is not configured")
	}
	ids := make([]string, 0, len(tasks))
	state := make(map[string]*taskGraphNodeState, len(tasks))
	for _, task := range tasks {
		taskID := strings.TrimSpace(task.ID)
		if taskID == "" {
			continue
		}
		ids = append(ids, taskID)
		state[taskID] = &taskGraphNodeState{
			TaskID:      taskID,
			Description: strings.TrimSpace(task.Description),
			Status:      message.ToolResultSubtaskStatusPending,
			Messages:    []string{},
		}
	}
	if err := service.Open(mailboxID, ids); err != nil {
		return nil, err
	}
	bridge := &taskGraphMailboxBridge{
		service:   service,
		session:   sessions,
		mailbox:   strings.TrimSpace(mailboxID),
		sessionID: strings.TrimSpace(sessionID),
		state:     state,
	}
	bridge.syncTodos(context.Background())
	return bridge, nil
}

func (b *taskGraphMailboxBridge) Close() {
	if b == nil {
		return
	}
	b.service.Close(b.mailbox)
}

func (b *taskGraphMailboxBridge) UpdateProgress(taskID string, toolUses int, lastTool string) {
	if b == nil {
		return
	}
	id := strings.TrimSpace(taskID)
	b.mu.Lock()
	node, ok := b.state[id]
	if !ok {
		b.mu.Unlock()
		return
	}
	node.ToolUses = toolUses
	if lastTool != "" {
		node.LastTool = lastTool
	}
	b.mu.Unlock()
	b.syncTodos(context.Background())
}

func (b *taskGraphMailboxBridge) MarkPending(taskID string) {
	b.update(taskID, message.ToolResultSubtaskStatusPending, "")
}

func (b *taskGraphMailboxBridge) MarkInProgress(taskID string) {
	b.update(taskID, message.ToolResultSubtaskStatusInProgress, "")
}

func (b *taskGraphMailboxBridge) MarkResult(taskID string, status message.ToolResultSubtaskStatus, content string) {
	b.update(taskID, status, content)
}

func (b *taskGraphMailboxBridge) Consume(taskID string) (taskGraphMailboxEffects, error) {
	if b == nil {
		return taskGraphMailboxEffects{}, nil
	}
	envelopes, err := b.service.Consume(b.mailbox, taskID)
	if err != nil {
		return taskGraphMailboxEffects{}, err
	}
	if len(envelopes) == 0 {
		return taskGraphMailboxEffects{}, nil
	}

	effects := taskGraphMailboxEffects{}
	messages := make([]string, 0, len(envelopes))
	for _, envelope := range envelopes {
		switch envelope.Kind {
		case mailbox.EnvelopeKindMessage:
			if envelope.Message != "" {
				messages = append(messages, envelope.Message)
			}
		case mailbox.EnvelopeKindStop:
			effects.Stop = true
			effects.Reason = strings.TrimSpace(envelope.Reason)
		}
	}
	if len(messages) > 0 {
		effects.Messages = append(effects.Messages, messages...)
		b.mu.Lock()
		if node, ok := b.state[strings.TrimSpace(taskID)]; ok {
			node.Messages = append(node.Messages, messages...)
		}
		b.mu.Unlock()
		b.syncTodos(context.Background())
	}
	if effects.Stop {
		if effects.Reason == "" {
			effects.Reason = "Task stop requested via mailbox."
		}
		b.update(taskID, message.ToolResultSubtaskStatusCanceled, effects.Reason)
	}
	return effects, nil
}

func (b *taskGraphMailboxBridge) update(taskID string, status message.ToolResultSubtaskStatus, content string) {
	if b == nil {
		return
	}
	id := strings.TrimSpace(taskID)
	b.mu.Lock()
	node, ok := b.state[id]
	if !ok {
		b.mu.Unlock()
		return
	}
	node.Status = status
	trimmed := strings.TrimSpace(content)
	if trimmed != "" {
		node.Content = trimmed
	}
	b.mu.Unlock()
	b.syncTodos(context.Background())
}

func (b *taskGraphMailboxBridge) syncTodos(ctx context.Context) {
	if b == nil || b.session == nil || b.sessionID == "" {
		return
	}
	sess, err := b.session.Get(ctx, b.sessionID)
	if err != nil {
		return
	}

	b.mu.Lock()
	nodes := make([]*taskGraphNodeState, 0, len(b.state))
	for _, node := range b.state {
		nodes = append(nodes, &taskGraphNodeState{
			TaskID:      node.TaskID,
			Description: node.Description,
			Status:      node.Status,
			Content:     node.Content,
			Messages:    append([]string(nil), node.Messages...),
			ToolUses:    node.ToolUses,
			LastTool:    node.LastTool,
		})
	}
	b.mu.Unlock()

	todos := make([]session.Todo, 0, len(nodes))
	for _, node := range nodes {
		todos = append(todos, session.Todo{
			ID:         node.TaskID,
			Content:    taskGraphTodoContent(node),
			Status:     taskGraphTodoStatus(node.Status),
			Progress:   taskGraphTodoProgress(node.Status, node.ToolUses),
			ActiveForm: taskGraphTodoActiveForm(node),
		})
	}
	sess.Todos = todos
	_, _ = b.session.Save(ctx, sess)
}

func taskGraphTodoStatus(status message.ToolResultSubtaskStatus) session.TodoStatus {
	switch status {
	case message.ToolResultSubtaskStatusInProgress:
		return session.TodoStatusInProgress
	case message.ToolResultSubtaskStatusCompleted:
		return session.TodoStatusCompleted
	case message.ToolResultSubtaskStatusFailed:
		return session.TodoStatusFailed
	case message.ToolResultSubtaskStatusCanceled:
		return session.TodoStatusCanceled
	default:
		return session.TodoStatusPending
	}
}

func taskGraphTodoProgress(status message.ToolResultSubtaskStatus, toolUses int) int {
	switch status {
	case message.ToolResultSubtaskStatusCompleted, message.ToolResultSubtaskStatusFailed, message.ToolResultSubtaskStatusCanceled:
		return 100
	case message.ToolResultSubtaskStatusInProgress:
		return min(95, 10+toolUses*5)
	default:
		return 0
	}
}

func taskGraphTodoActiveForm(node *taskGraphNodeState) string {
	if node == nil {
		return ""
	}
	switch node.Status {
	case message.ToolResultSubtaskStatusInProgress:
		if node.LastTool != "" {
			return fmt.Sprintf("Running (%s)", node.LastTool)
		}
		return "Running"
	case message.ToolResultSubtaskStatusFailed:
		return "Failed"
	case message.ToolResultSubtaskStatusCanceled:
		return "Canceled"
	case message.ToolResultSubtaskStatusCompleted:
		return "Completed"
	default:
		return "Pending"
	}
}

func taskGraphTodoContent(node *taskGraphNodeState) string {
	if node == nil {
		return ""
	}
	base := strings.TrimSpace(node.Description)
	if base == "" {
		base = node.TaskID
	}
	if content := taskGraphCompactText(node.Content); content != "" {
		content, truncated := taskGraphEllipsize(content, taskGraphTodoNodeContentCharsLimit)
		if truncated {
			content += " [truncated]"
		}
		base = fmt.Sprintf("%s (%s)", base, content)
	}
	if len(node.Messages) > 0 {
		mailboxMessage := taskGraphCompactText(node.Messages[len(node.Messages)-1])
		if mailboxMessage != "" {
			mailboxMessage, _ = taskGraphEllipsize(mailboxMessage, taskGraphTodoMailboxCharsLimit)
			base = fmt.Sprintf("%s [mailbox:%s]", base, mailboxMessage)
		}
	}
	trimmed, _ := taskGraphEllipsize(base, taskGraphTodoContentCharsLimit)
	return trimmed
}

type taskGraphMailboxEffects struct {
	Messages []string
	Stop     bool
	Reason   string
}
