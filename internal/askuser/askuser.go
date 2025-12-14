// Package askuser provides a service for tools to ask users questions
// and block until they respond. It follows the same pattern as the
// permission service.
package askuser

import (
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// QuestionOption represents a single choice option for a question.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Question represents a single question to ask the user.
type Question struct {
	Question    string           `json:"question"`
	Header      string           `json:"header"`
	Options     []QuestionOption `json:"options"`
	MultiSelect bool             `json:"multi_select,omitempty"`
}

// Answer represents the user's response to a question.
type Answer struct {
	QuestionIndex   int      `json:"question_index"`
	SelectedIndices []int    `json:"selected_indices"`
	SelectedIndex   int      `json:"selected_index"`
	OtherText       string   `json:"other_text,omitempty"`
	IsOther         bool     `json:"is_other,omitempty"`
}

// CreateAskUserRequest is used to create a new ask user request.
type CreateAskUserRequest struct {
	SessionID  string     `json:"session_id"`
	ToolCallID string     `json:"tool_call_id"`
	Questions  []Question `json:"questions"`
}

// AskUserRequest is the request published via pubsub for the UI to handle.
type AskUserRequest struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"session_id"`
	ToolCallID string     `json:"tool_call_id"`
	Questions  []Question `json:"questions"`
}

// AskUserResponse contains all user answers.
type AskUserResponse struct {
	RequestID string   `json:"request_id"`
	Answers   []Answer `json:"answers"`
	Cancelled bool     `json:"cancelled"`
}

// Service defines the askuser service interface.
type Service interface {
	pubsub.Subscriber[AskUserRequest]
	// Request blocks until user responds and returns their answers.
	Request(opts CreateAskUserRequest) (*AskUserResponse, error)
	// Respond unblocks the waiting tool with the user's answers.
	Respond(requestID string, response AskUserResponse)
	// Cancel cancels a pending request.
	Cancel(requestID string)
}

type askUserService struct {
	*pubsub.Broker[AskUserRequest]
	pendingRequests *csync.Map[string, chan AskUserResponse]
	requestMu       sync.Mutex
	activeRequest   *AskUserRequest
}

// NewService creates a new askuser service.
func NewService() Service {
	return &askUserService{
		Broker:          pubsub.NewBroker[AskUserRequest](),
		pendingRequests: csync.NewMap[string, chan AskUserResponse](),
	}
}

// Request blocks until user responds or context is cancelled.
func (s *askUserService) Request(opts CreateAskUserRequest) (*AskUserResponse, error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	request := AskUserRequest{
		ID:         uuid.New().String(),
		SessionID:  opts.SessionID,
		ToolCallID: opts.ToolCallID,
		Questions:  opts.Questions,
	}

	s.activeRequest = &request

	respCh := make(chan AskUserResponse, 1)
	s.pendingRequests.Set(request.ID, respCh)
	defer func() {
		s.pendingRequests.Del(request.ID)
		if s.activeRequest != nil && s.activeRequest.ID == request.ID {
			s.activeRequest = nil
		}
	}()

	// Publish request for TUI to handle
	s.Publish(pubsub.CreatedEvent, request)

	// Block waiting for response
	response := <-respCh
	return &response, nil
}

// Respond unblocks the waiting tool with the user's answers.
func (s *askUserService) Respond(requestID string, response AskUserResponse) {
	respCh, ok := s.pendingRequests.Get(requestID)
	if ok {
		response.RequestID = requestID
		respCh <- response
	}

	if s.activeRequest != nil && s.activeRequest.ID == requestID {
		s.activeRequest = nil
	}
}

// Cancel cancels a pending request.
func (s *askUserService) Cancel(requestID string) {
	respCh, ok := s.pendingRequests.Get(requestID)
	if ok {
		respCh <- AskUserResponse{
			RequestID: requestID,
			Cancelled: true,
		}
	}

	if s.activeRequest != nil && s.activeRequest.ID == requestID {
		s.activeRequest = nil
	}
}
