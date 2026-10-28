// Package question provides services for asking the user questions
// via the TUI and blocking until an answer is received. It mirrors
// the permission service pattern: publish a request over pubsub,
// block on a channel, and resolve when the UI sends back answers.
//
// A session asks one question at a time (the tool blocks until
// answered), but several sessions can ask at once, so answers and
// cancellations name the batch they resolve by its request ID.
package question

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// ErrCancelled is returned by Ask when the user cancels the question.
var ErrCancelled = errors.New("question cancelled by user")

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
	for i, q := range r.Questions {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("question %d: %w", i+1, err)
		}
	}
	return nil
}

// Validate checks that a Question has valid fields. Error messages
// are written for LLM consumption: specific and actionable.
func (q Question) Validate() error {
	label := q.identifier()
	if q.Text == "" {
		return fmt.Errorf("%s: question text is required", label)
	}
	if len(q.Text) > MaxQuestionLength {
		return fmt.Errorf("%s: text exceeds %d characters (got %d)", label, MaxQuestionLength, len(q.Text))
	}
	if q.Description == "" {
		return fmt.Errorf("%s: description is required", label)
	}
	if len(q.Description) > MaxDescriptionLength {
		return fmt.Errorf("%s: description exceeds %d characters (got %d)", label, MaxDescriptionLength, len(q.Description))
	}
	switch q.Type {
	case TypeYesNo, TypeFreeText:
		// No choices needed.
	case TypeSingleChoice, TypeMultiChoice:
		if len(q.Choices) < 2 {
			return fmt.Errorf("%s: %s requires at least 2 choices in the \"choices\" array (got %d). Use \"choices\", not \"options\"", label, q.Type, len(q.Choices))
		}
		if len(q.Choices) > MaxChoices {
			return fmt.Errorf("%s: choices exceed maximum of %d (got %d)", label, MaxChoices, len(q.Choices))
		}
		seen := make(map[string]bool, len(q.Choices))
		for i, c := range q.Choices {
			if c.ID == "" {
				return fmt.Errorf("%s: choice %d must have an \"id\" field", label, i+1)
			}
			if seen[c.ID] {
				return fmt.Errorf("%s: choice %d has duplicate id %q", label, i+1, c.ID)
			}
			seen[c.ID] = true
			if c.Label == "" {
				return fmt.Errorf("%s: choice %d (%s) must have a \"label\" field", label, i+1, c.ID)
			}
			if len(c.Label) > MaxChoiceLabelLength {
				return fmt.Errorf("%s: choice %d label exceeds %d characters (got %d)", label, i+1, MaxChoiceLabelLength, len(c.Label))
			}
			if len(c.Description) > MaxChoiceDescriptionLength {
				return fmt.Errorf("%s: choice %d description exceeds %d characters (got %d)", label, i+1, MaxChoiceDescriptionLength, len(c.Description))
			}
		}
	default:
		return fmt.Errorf("%s: unknown type %q (must be yes_no, single_choice, multi_choice, or free_text)", label, q.Type)
	}
	return nil
}

// identifier returns a human-readable label for error messages.
// Uses the question label, text excerpt, or a fallback.
func (q Question) identifier() string {
	if q.Label != "" {
		return fmt.Sprintf("[%s]", q.Label)
	}
	if q.Text != "" {
		t := q.Text
		if len(t) > 40 {
			t = t[:40] + "…"
		}
		return fmt.Sprintf("[%s]", t)
	}
	return "[unnamed question]"
}

const (
	MaxQuestionLength          = 240
	MaxDescriptionLength       = 600
	MaxChoiceLabelLength       = 200
	MaxChoiceDescriptionLength = 200
	MaxChoices                 = 5
	MaxQuestions               = 5
)

// Notification is published when a question batch is resolved so
// that non-answering clients can dismiss their open forms.
type Notification struct {
	BatchID   string `json:"batch_id"`
	SessionID string `json:"session_id"`
}

// Service manages the lifecycle of question requests. Several
// sessions can each have a batch pending at once, so answers and
// cancellations name the batch they resolve.
type Service interface {
	pubsub.Subscriber[Request]

	// SubscribeNotifications returns a channel for question
	// resolution notifications.
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[Notification]

	// Ask publishes questions and blocks until the user answers
	// or the context is cancelled.
	Ask(ctx context.Context, req Request) ([]Answer, error)

	// Answer delivers answers to the pending batch with the given ID.
	// An empty ID names the only pending batch, for callers that
	// predate batch IDs. Returns false if no such batch is pending.
	Answer(batchID string, answers []Answer) bool

	// Cancel cancels the pending batch with the given ID, treating an
	// empty ID as Answer does. Returns false if no such batch is
	// pending.
	Cancel(batchID string) bool

	// Pending returns the requests that are currently awaiting an
	// answer, in no particular order.
	Pending() []Request
}

type questionService struct {
	broker             *pubsub.Broker[Request]
	notificationBroker *pubsub.Broker[Notification]
	mu                 sync.Mutex
	pending            map[string]*pendingBatch
}

// pendingBatch is a request awaiting an answer, with the channels its
// Ask call waits on.
type pendingBatch struct {
	req       Request
	answers   chan []Answer
	cancelled chan struct{}
}

// NewService creates a new question service.
func NewService() *questionService {
	return &questionService{
		broker:             pubsub.NewBroker[Request](),
		notificationBroker: pubsub.NewBroker[Notification](),
		pending:            make(map[string]*pendingBatch),
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

	// Apply defaults for multi-question confirm fields.
	if len(req.Questions) >= 2 {
		if req.ConfirmTitle == "" {
			req.ConfirmTitle = "Ready to go?"
		}
		if req.ConfirmDescription == "" {
			req.ConfirmDescription = "Review your answers above and confirm."
		}
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	p := &pendingBatch{
		req:       req,
		answers:   make(chan []Answer, 1),
		cancelled: make(chan struct{}),
	}
	s.mu.Lock()
	if _, ok := s.pending[req.ID]; ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("question batch %q is already pending", req.ID)
	}
	s.pending[req.ID] = p
	s.mu.Unlock()

	defer func() {
		// A resolver that took the batch already removed it, and its
		// ID may since have been reused.
		s.mu.Lock()
		if s.pending[req.ID] == p {
			delete(s.pending, req.ID)
		}
		s.mu.Unlock()
	}()

	s.broker.Publish(pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.cancelled:
		return nil, ErrCancelled
	case answers := <-p.answers:
		return answers, nil
	}
}

// take removes the pending batch with the given ID and returns it.
// Removing it under the lock makes the caller its only resolver. An
// empty ID takes the only pending batch; with several pending it is
// ambiguous and takes none.
func (s *questionService) take(batchID string) (*pendingBatch, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if batchID == "" && len(s.pending) == 1 {
		for id := range s.pending {
			batchID = id
		}
	}
	p, ok := s.pending[batchID]
	if ok {
		delete(s.pending, batchID)
	}
	return p, ok
}

// Answer resolves the pending batch with the given ID. Returns false
// if no such batch is pending (already answered or cancelled).
func (s *questionService) Answer(batchID string, answers []Answer) bool {
	p, ok := s.take(batchID)
	if !ok {
		return false
	}
	// The channel is buffered and take made this its only sender, so
	// this never blocks.
	p.answers <- answers

	// Publish a notification so non-answering clients can dismiss
	// their open question forms.
	s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{
		BatchID:   p.req.ID,
		SessionID: p.req.SessionID,
	})
	return true
}

// Cancel cancels the pending batch with the given ID. Returns false
// if no such batch is pending.
func (s *questionService) Cancel(batchID string) bool {
	p, ok := s.take(batchID)
	if !ok {
		return false
	}
	close(p.cancelled)

	// Publish a notification so non-answering clients can dismiss
	// their open question forms.
	s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{
		BatchID:   p.req.ID,
		SessionID: p.req.SessionID,
	})
	return true
}

// Pending returns the requests that are currently awaiting an answer,
// in no particular order.
func (s *questionService) Pending() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	reqs := make([]Request, 0, len(s.pending))
	for _, p := range s.pending {
		reqs = append(reqs, p.req)
	}
	return reqs
}
