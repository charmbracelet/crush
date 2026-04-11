package questions

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Option represents a single answer option for a question.
type Option struct {
	Label       string `json:"label" description:"Short label identifying the option"`
	Description string `json:"description" description:"Optional description of the option"`
}

// Question represents a single question to be asked.
type Question struct {
	ID       string `json:"id" description:"UUID of originating question: used to correlate question and answer"`
	Question string `json:"question" description:"Short question to ask the user"`

	// Options are the possible Answer(s) to the question.
	Options []Option `json:"options" description:"Array of options from which user will select one or more answers"`

	// MultiSelect indicates whether the user can Answer by selecting multiple Option(s).
	MultiSelect bool `json:"multi_select" description:"Indicates if user can answer selecting multiple options"`
}

// QuestionsRequest represents a request to ask a set of Question(s).
type QuestionsRequest struct {
	ID         string
	SessionID  string
	ToolCallID string
	Questions  []Question
}

// NewQuestionsRequest creates a new QuestionsRequest.
func NewQuestionsRequest(sessionID string, toolCallID string, questions []Question) QuestionsRequest {
	return QuestionsRequest{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		ToolCallID: toolCallID,
		Questions:  questions,
	}
}

// Answer represents a user's response to a Question.
type Answer struct {
	// ID is the ID of the original Question.
	// This ensures that even if the LLM asks multiple Question(s),
	// the Answer(s) can be easily correlated to the original Question.
	ID string `json:"id" description:"UUID of originating question"`

	// Selected is the index of the selected Option.
	Selected []string `json:"selected" description:"Array of options' labels selected by user"`
}

func NewAnswer(question Question) Answer {
	return Answer{
		ID:       question.ID,
		Selected: []string{},
	}
}

// Select set one or more user-selection Option(s) to the Answer.
func (ans *Answer) Select(label ...string) {
	ans.Selected = append(ans.Selected, label...)
}

// QuestionsResponse represents the set of Answer(s) to a set of Question(s)
// contained in the originating QuestionsRequest.
// The originating QuestionsRequest is identified by the RequestID.
type QuestionsResponse struct {
	RequestID string   `json:"request_id"`
	Answers   []Answer `json:"answers"`
}

func NewQuestionsResponse(req *QuestionsRequest) QuestionsResponse {
	return QuestionsResponse{
		RequestID: req.ID,
		Answers:   make([]Answer, len(req.Questions)),
	}
}

// SetAnswerAt sets the Answer at the given index.
func (res *QuestionsResponse) SetAnswerAt(idx int, ans Answer) {
	res.Answers[idx] = ans
}

// IsComplete returns true if the QuestionsResponse contains all the expected Answer(s).
func (res *QuestionsResponse) IsComplete() bool {
	return len(res.Answers) == cap(res.Answers)
}

// Service is the interface for the questionsService.
// When Ask is invoked, a new QuestionsRequest is published to the service.
// When the user answers the Question(s), the Answer(s) are sent back via the Answer method.
type Service interface {
	pubsub.Subscriber[QuestionsRequest]

	Ask(ctx context.Context, req QuestionsRequest) (QuestionsResponse, error)
	Answer(response QuestionsResponse)
}

// questionsService is a pubsub.Broker[QuestionsRequest] that tracks the pending
// QuestionsResponse channels for each submitted QuestionsRequest.
type questionsService struct {
	*pubsub.Broker[QuestionsRequest]

	// pendingRequests maps a QuestionsRequest.ID to a channel that will be used to
	// send the QuestionsResponse back when the user answers the Question(s).
	pendingRequests *csync.Map[string, chan QuestionsResponse]
}

// NewService creates a new questionsService.
func NewService() Service {
	return &questionsService{
		Broker:          pubsub.NewBroker[QuestionsRequest](),
		pendingRequests: csync.NewMap[string, chan QuestionsResponse](),
	}
}

func (s *questionsService) Ask(ctx context.Context, req QuestionsRequest) (QuestionsResponse, error) {
	slog.Debug("Ask", "request_id", req.ID, "session_id", req.SessionID, "tool_call_id", req.ToolCallID, "questions", len(req.Questions))
	ch := make(chan QuestionsResponse, 1)
	s.pendingRequests.Set(req.ID, ch)
	defer s.pendingRequests.Del(req.ID)

	s.Publish(pubsub.CreatedEvent, req)

	select {
	// If the context is cancelled, return and empty AnswersResponse and the error.
	case <-ctx.Done():
		slog.Debug("Ask cancelled", "request_id", req.ID)
		return QuestionsResponse{RequestID: req.ID}, ctx.Err()
	// Otherwise, wait for the user to answer the Question(s).
	case resp := <-ch:
		return resp, nil
	}
}

func (s *questionsService) Answer(res QuestionsResponse) {
	if ch, found := s.pendingRequests.Get(res.RequestID); found {
		slog.Debug("Answer", "request_id", res.RequestID, "answers", res.Answers)
		ch <- res
	} else {
		slog.Warn("Received answers for unknown questions", "request_id", res.RequestID, "answers", res.Answers)
	}
}
