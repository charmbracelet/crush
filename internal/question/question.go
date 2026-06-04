// Package question provides services for asking the user questions
// via the TUI and blocking until an answer is received. It mirrors
// the permission service pattern: publish a request over pubsub,
// block on a channel, and resolve when the UI sends back answers.
//
// Only one question can be pending at a time (the tool blocks until
// answered), so no correlation IDs are needed in the domain model.
package question

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Type identifies the kind of question to present.
type Type string

const (
	TypeYesNo        Type = "yes_no"
	TypeSingleChoice Type = "single_choice"
	TypeMultiChoice  Type = "multi_choice"
	TypeFreeText     Type = "free_text"
)

// Choice represents a single selectable option.
type Choice struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Question is a single question definition within a Request.
type Question struct {
	ID          string   `json:"id"`
	Type        Type     `json:"type"`
	Label       string   `json:"label,omitempty"`
	Text        string   `json:"question"`
	Description string   `json:"description,omitempty"`
	Choices     []Choice `json:"choices,omitempty"`
}

// Answer carries the user's response to a single Question.
type Answer struct {
	QuestionID  string            `json:"question_id"`
	SelectedIDs []string          `json:"selected_ids,omitempty"`
	FillInText  string            `json:"fill_in_text,omitempty"`
	Yes         *bool             `json:"yes,omitempty"`
	Notes       map[string]string `json:"notes,omitempty"`
}

// HasNotes reports whether any notes were attached.
func (a Answer) HasNotes() bool { return len(a.Notes) > 0 }

// Request is the service envelope published to the UI. It contains
// one or more Questions. A single question renders without tabs;
// multiple questions render as a tabbed form with confirmation.
type Request struct {
	ID                 string     `json:"id"`
	SessionID          string     `json:"session_id"`
	ToolCallID         string     `json:"tool_call_id"`
	Questions          []Question `json:"questions"`
	ConfirmTitle       string     `json:"confirm_title,omitempty"`
	ConfirmDescription string     `json:"confirm_description,omitempty"`
}

// Validate checks that a Request has valid fields. For multiple
// questions, ConfirmTitle and ConfirmDescription are required.
func (r Request) Validate() error {
	if len(r.Questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}
	if len(r.Questions) > MaxQuestions {
		return fmt.Errorf("questions exceed maximum of %d (got %d)", MaxQuestions, len(r.Questions))
	}
	if len(r.Questions) >= 2 {
		if r.ConfirmTitle == "" {
			return fmt.Errorf("confirm_title is required for multi-question requests")
		}
		if r.ConfirmDescription == "" {
			return fmt.Errorf("confirm_description is required for multi-question requests")
		}
	}
	for i, q := range r.Questions {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("question %d: %w", i, err)
		}
	}
	return nil
}

// Validate checks that a Question has valid fields. Error messages
// are written for LLM consumption: specific and actionable.
func (q Question) Validate() error {
	if q.Text == "" {
		return fmt.Errorf("question text is required")
	}
	if len(q.Text) > MaxQuestionLength {
		return fmt.Errorf("text exceeds %d characters (got %d)", MaxQuestionLength, len(q.Text))
	}
	if q.Description == "" {
		return fmt.Errorf("description is required")
	}
	if len(q.Description) > MaxDescriptionLength {
		return fmt.Errorf("description exceeds %d characters (got %d)", MaxDescriptionLength, len(q.Description))
	}
	switch q.Type {
	case TypeYesNo, TypeFreeText:
		// No choices needed.
	case TypeSingleChoice, TypeMultiChoice:
		if len(q.Choices) < 2 {
			return fmt.Errorf("%s requires at least 2 choices (got %d)", q.Type, len(q.Choices))
		}
		if len(q.Choices) > MaxChoices {
			return fmt.Errorf("choices exceed maximum of %d (got %d)", MaxChoices, len(q.Choices))
		}
		seen := make(map[string]bool, len(q.Choices))
		for i, c := range q.Choices {
			if c.ID == "" {
				return fmt.Errorf("choice %d must have an id", i)
			}
			if seen[c.ID] {
				return fmt.Errorf("choice %d has duplicate id %q", i, c.ID)
			}
			seen[c.ID] = true
			if c.Label == "" {
				return fmt.Errorf("choice %d must have a label", i)
			}
			if len(c.Label) > MaxChoiceLabelLength {
				return fmt.Errorf("choice %d label exceeds %d characters (got %d)", i, MaxChoiceLabelLength, len(c.Label))
			}
			if len(c.Description) > MaxChoiceDescriptionLength {
				return fmt.Errorf("choice %d description exceeds %d characters (got %d)", i, MaxChoiceDescriptionLength, len(c.Description))
			}
		}
	default:
		return fmt.Errorf("unknown type %q (must be yes_no, single_choice, multi_choice, or free_text)", q.Type)
	}
	return nil
}

const (
	MaxQuestionLength          = 120
	MaxDescriptionLength       = 300
	MaxChoiceLabelLength       = 120
	MaxChoiceDescriptionLength = 100
	MaxChoices                 = 5
	MaxQuestions               = 5
)

// Notification is published when a question batch is resolved so
// that non-answering clients can dismiss their open forms.
type Notification struct {
	BatchID string `json:"batch_id"`
}

// Service manages the lifecycle of question requests. Only one
// question can be pending at a time.
type Service interface {
	pubsub.Subscriber[Request]

	// SubscribeNotifications returns a channel for question
	// resolution notifications.
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[Notification]

	// Ask publishes questions and blocks until the user answers
	// or the context is cancelled.
	Ask(ctx context.Context, req Request) ([]Answer, error)

	// Answer resolves the pending question with the given answers.
	Answer(answers []Answer) bool
}

type questionService struct {
	broker             *pubsub.Broker[Request]
	notificationBroker *pubsub.Broker[Notification]
	mu                 sync.Mutex
	pending            chan []Answer
	pendingID          string
}

// NewService creates a new question service.
func NewService() *questionService {
	return &questionService{
		broker:             pubsub.NewBroker[Request](),
		notificationBroker: pubsub.NewBroker[Notification](),
	}
}

// Subscribe returns a channel for question events.
func (s *questionService) Subscribe(ctx context.Context) <-chan pubsub.Event[Request] {
	return s.broker.Subscribe(ctx)
}

// SubscribeNotifications returns a channel for question resolution
// notifications.
func (s *questionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[Notification] {
	return s.notificationBroker.Subscribe(ctx)
}

// Ask publishes a request and blocks until the user answers.
func (s *questionService) Ask(ctx context.Context, req Request) ([]Answer, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	for i := range req.Questions {
		if req.Questions[i].ID == "" {
			req.Questions[i].ID = uuid.New().String()
		}
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.pending = make(chan []Answer, 1)
	s.pendingID = req.ID
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.pending = nil
		s.pendingID = ""
		s.mu.Unlock()
	}()

	s.broker.Publish(pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case answers := <-s.pending:
		return answers, nil
	}
}

// Answer resolves the pending question. Returns false if no
// question is pending (already answered or cancelled).
func (s *questionService) Answer(answers []Answer) bool {
	s.mu.Lock()
	batchID := s.pendingID
	ch := s.pending
	s.mu.Unlock()

	if ch == nil {
		return false
	}
	ch <- answers

	// Publish a notification so non-answering clients can dismiss
	// their open question forms.
	if batchID != "" {
		s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{
			BatchID: batchID,
		})
	}
	return true
}
