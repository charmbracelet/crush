package question

import (
	"context"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// QuestionRequest is the event published to the UI when the model wants
// to ask the user a clarifying question.
type QuestionRequest struct {
	ID            string   `json:"id"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	AllowMultiple bool     `json:"allow_multiple,omitempty"`
}

// QuestionResponse is the answer submitted by the user for a question.
type QuestionResponse struct {
	Answers []string
}

// Service is the interface the tool and UI both depend on.
type Service interface {
	pubsub.Subscriber[QuestionRequest]
	// Ask blocks until the user responds or ctx is cancelled.
	Ask(ctx context.Context, question string, options []string, allowMultiple bool) (QuestionResponse, error)
	// Respond unblocks a pending Ask call with the given response.
	Respond(id string, response QuestionResponse)
}

type questionService struct {
	*pubsub.Broker[QuestionRequest]

	pendingRequests *csync.Map[string, chan QuestionResponse]
	// serializes concurrent Ask calls so only one question is shown at a time.
	requestMu sync.Mutex
}

func NewService() Service {
	return &questionService{
		Broker:          pubsub.NewBroker[QuestionRequest](),
		pendingRequests: csync.NewMap[string, chan QuestionResponse](),
	}
}

func (s *questionService) Ask(ctx context.Context, question string, options []string, allowMultiple bool) (QuestionResponse, error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	req := QuestionRequest{
		ID:            uuid.New().String(),
		Question:      question,
		Options:       options,
		AllowMultiple: allowMultiple,
	}

	respCh := make(chan QuestionResponse, 1)
	s.pendingRequests.Set(req.ID, respCh)
	defer s.pendingRequests.Del(req.ID)

	s.Publish(pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		// Tell the UI to dismiss the dialog for this now-abandoned request.
		s.Publish(pubsub.DeletedEvent, req)
		return QuestionResponse{}, ctx.Err()
	case response := <-respCh:
		return response, nil
	}
}

func (s *questionService) Respond(id string, response QuestionResponse) {
	ch, ok := s.pendingRequests.Get(id)
	if ok {
		// respCh is buffered (1); the non-blocking send guards against a
		// duplicate Respond for the same id stalling the caller.
		select {
		case ch <- response:
		default:
		}
	}
}
