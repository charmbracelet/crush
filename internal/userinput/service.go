package userinput

import (
	"context"
	"errors"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

const RequestTimeoutReason = "timed_out"

type ResponseStatus string

const (
	ResponseStatusSubmitted ResponseStatus = "submitted"
	ResponseStatusCanceled  ResponseStatus = "canceled"
	ResponseStatusTimedOut  ResponseStatus = "timed_out"
)

type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type Question struct {
	Header   string   `json:"header"`
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Options  []Option `json:"options"`
}

type CreateRequest struct {
	SessionID  string     `json:"session_id"`
	ToolCallID string     `json:"tool_call_id"`
	Questions  []Question `json:"questions"`
}

type Request struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"session_id"`
	ToolCallID string     `json:"tool_call_id"`
	Questions  []Question `json:"questions"`
}

type Answer struct {
	QuestionID     string `json:"question_id"`
	SelectedOption string `json:"selected_option,omitempty"`
	CustomInput    string `json:"custom_input,omitempty"`
}

type Response struct {
	RequestID    string         `json:"request_id"`
	SessionID    string         `json:"session_id"`
	ToolCallID   string         `json:"tool_call_id"`
	Status       ResponseStatus `json:"status"`
	Answers      []Answer       `json:"answers"`
	CancelReason string         `json:"cancel_reason,omitempty"`
}

type Service interface {
	pubsub.Subscriber[Request]
	Request(ctx context.Context, req CreateRequest) (Response, error)
	Resolve(response Response)
}

type service struct {
	*pubsub.Broker[Request]

	pendingRequests *csync.Map[string, chan Response]
}

func NewService() Service {
	return &service{
		Broker:          pubsub.NewBroker[Request](),
		pendingRequests: csync.NewMap[string, chan Response](),
	}
}

func (s *service) Request(ctx context.Context, req CreateRequest) (Response, error) {
	if req.SessionID == "" {
		return Response{}, errors.New("session_id is required")
	}
	if req.ToolCallID == "" {
		return Response{}, errors.New("tool_call_id is required")
	}

	request := Request{
		ID:         uuid.NewString(),
		SessionID:  req.SessionID,
		ToolCallID: req.ToolCallID,
		Questions:  req.Questions,
	}
	respCh := make(chan Response, 1)
	s.pendingRequests.Set(request.ID, respCh)
	defer s.pendingRequests.Del(request.ID)

	s.Publish(pubsub.CreatedEvent, request)

	select {
	case <-ctx.Done():
		status := ResponseStatusCanceled
		cancelReason := "canceled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = ResponseStatusTimedOut
			cancelReason = RequestTimeoutReason
		}
		return Response{
			RequestID:    request.ID,
			SessionID:    request.SessionID,
			ToolCallID:   request.ToolCallID,
			Status:       status,
			CancelReason: cancelReason,
		}, nil
	case response := <-respCh:
		return response, nil
	}
}

func (s *service) Resolve(response Response) {
	respCh, ok := s.pendingRequests.Get(response.RequestID)
	if !ok {
		return
	}
	respCh <- response
}
