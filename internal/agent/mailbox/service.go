package mailbox

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type EnvelopeKind string

const (
	EnvelopeKindMessage EnvelopeKind = "message"
	EnvelopeKindStop    EnvelopeKind = "stop"
)

type Envelope struct {
	MailboxID    string       `json:"mailbox_id"`
	TargetTaskID string       `json:"target_task_id,omitempty"`
	Kind         EnvelopeKind `json:"kind"`
	Message      string       `json:"message,omitempty"`
	Reason       string       `json:"reason,omitempty"`
	CreatedAt    int64        `json:"created_at"`
}

type Service interface {
	Open(mailboxID string, taskIDs []string) error
	Close(mailboxID string)
	Send(mailboxID, taskID, message string) (Envelope, error)
	Stop(mailboxID, taskID, reason string) (Envelope, error)
	Consume(mailboxID, taskID string) ([]Envelope, error)
}

type service struct {
	mu        sync.Mutex
	mailboxes map[string]*mailbox
}

type mailbox struct {
	tasks  map[string]struct{}
	queues map[string][]Envelope
}

func NewService() Service {
	return &service{mailboxes: map[string]*mailbox{}}
}

func (s *service) Open(mailboxID string, taskIDs []string) error {
	id := strings.TrimSpace(mailboxID)
	if id == "" {
		return fmt.Errorf("mailbox_id is required")
	}
	if len(taskIDs) == 0 {
		return fmt.Errorf("task_ids is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make(map[string]struct{}, len(taskIDs))
	queues := make(map[string][]Envelope, len(taskIDs))
	for _, taskID := range taskIDs {
		trimmed := strings.TrimSpace(taskID)
		if trimmed == "" {
			continue
		}
		tasks[trimmed] = struct{}{}
		queues[trimmed] = []Envelope{}
	}
	if len(tasks) == 0 {
		return fmt.Errorf("task_ids is required")
	}
	s.mailboxes[id] = &mailbox{tasks: tasks, queues: queues}
	return nil
}

func (s *service) Close(mailboxID string) {
	id := strings.TrimSpace(mailboxID)
	if id == "" {
		return
	}
	s.mu.Lock()
	delete(s.mailboxes, id)
	s.mu.Unlock()
}

func (s *service) Send(mailboxID, taskID, message string) (Envelope, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return Envelope{}, fmt.Errorf("message is required")
	}
	env := Envelope{
		MailboxID: strings.TrimSpace(mailboxID),
		Kind:      EnvelopeKindMessage,
		Message:   msg,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.enqueue(env, strings.TrimSpace(taskID)); err != nil {
		return Envelope{}, err
	}
	env.TargetTaskID = strings.TrimSpace(taskID)
	return env, nil
}

func (s *service) Stop(mailboxID, taskID, reason string) (Envelope, error) {
	env := Envelope{
		MailboxID: strings.TrimSpace(mailboxID),
		Kind:      EnvelopeKindStop,
		Reason:    strings.TrimSpace(reason),
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.enqueue(env, strings.TrimSpace(taskID)); err != nil {
		return Envelope{}, err
	}
	env.TargetTaskID = strings.TrimSpace(taskID)
	return env, nil
}

func (s *service) Consume(mailboxID, taskID string) ([]Envelope, error) {
	id := strings.TrimSpace(mailboxID)
	task := strings.TrimSpace(taskID)
	if id == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	if task == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	box, ok := s.mailboxes[id]
	if !ok {
		return nil, fmt.Errorf("mailbox %q not found", id)
	}
	if _, ok := box.tasks[task]; !ok {
		return nil, fmt.Errorf("task %q not found in mailbox %q", task, id)
	}
	queue := box.queues[task]
	if len(queue) == 0 {
		return nil, nil
	}
	out := append([]Envelope(nil), queue...)
	box.queues[task] = box.queues[task][:0]
	return out, nil
}

func (s *service) enqueue(envelope Envelope, taskID string) error {
	mailboxID := strings.TrimSpace(envelope.MailboxID)
	if mailboxID == "" {
		return fmt.Errorf("mailbox_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	box, ok := s.mailboxes[mailboxID]
	if !ok {
		return fmt.Errorf("mailbox %q not found", mailboxID)
	}

	if taskID != "" {
		if _, ok := box.tasks[taskID]; !ok {
			return fmt.Errorf("task %q not found in mailbox %q", taskID, mailboxID)
		}
		envelope.TargetTaskID = taskID
		box.queues[taskID] = append(box.queues[taskID], envelope)
		return nil
	}

	envelope.TargetTaskID = ""
	for currentTaskID := range box.tasks {
		box.queues[currentTaskID] = append(box.queues[currentTaskID], envelope)
	}
	return nil
}
