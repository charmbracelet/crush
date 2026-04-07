package ask_question

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Option represents a single answer option for a question.
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Question represents a single question to be asked.
type Question struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Header   string `json:"header"`

	// Options are the possible Answer(s) to the question.
	Options []Option `json:"options"`

	// MultiSelect indicates whether the user can Answer by selecting multiple Option(s).
	MultiSelect bool `json:"multi_select"`
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
	ID string `json:"id"`

	// Selected is the index of the selected Option.
	Selected []string `json:"selected"`
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

// AnswersResponse represents the set of Answer(s) to a set of Question(s)
// contained in the originating QuestionsRequest.
// The originating QuestionsRequest is identified by the RequestID.
type AnswersResponse struct {
	RequestID string   `json:"request_id"`
	Answers   []Answer `json:"answers"`
}

func NewAnswersResponse(req *QuestionsRequest) AnswersResponse {
	return AnswersResponse{
		RequestID: req.ID,
		Answers:   make([]Answer, len(req.Questions)),
	}
}

// SetAnswerAt sets the Answer at the given index.
func (res *AnswersResponse) SetAnswerAt(idx int, ans Answer) {
	res.Answers[idx] = ans
}

// IsComplete returns true if the AnswersResponse contains all the expected Answer(s).
func (res *AnswersResponse) IsComplete() bool {
	return len(res.Answers) == cap(res.Answers)
}

// Service is the interface for the AskQuestion service.
// When Ask is invoked, a new QuestionsRequest is published to the service.
// When the user answers the Question(s), the Answer(s) are sent back via the Answer method.
type Service interface {
	pubsub.Subscriber[QuestionsRequest]

	Ask(ctx context.Context, req QuestionsRequest) (AnswersResponse, error)
	Answer(response AnswersResponse)
}

// service is a pubsub.Broker[QuestionsRequest] that tracks the pending
// AnswersResponse channels for each submitted QuestionsRequest.
type service struct {
	*pubsub.Broker[QuestionsRequest]

	// pendingRequests maps a QuestionsRequest.ID to a channel that will be used to
	// send the AnswersResponse back when the user answers the Question(s).
	pendingRequests *csync.Map[string, chan AnswersResponse]
}

// NewService creates a new AskQuestion service.
func NewService() Service {
	return &service{
		Broker:          pubsub.NewBroker[QuestionsRequest](),
		pendingRequests: csync.NewMap[string, chan AnswersResponse](),
	}
}

func (s *service) Ask(ctx context.Context, req QuestionsRequest) (AnswersResponse, error) {
	slog.Debug("Ask", "request_id", req.ID, "session_id", req.SessionID, "tool_call_id", req.ToolCallID, "questions", len(req.Questions))
	ch := make(chan AnswersResponse, 1)
	s.pendingRequests.Set(req.ID, ch)
	defer s.pendingRequests.Del(req.ID)

	s.Publish(pubsub.CreatedEvent, req)

	select {
	// If the context is cancelled, return and empty AnswersResponse and the error.
	case <-ctx.Done():
		slog.Debug("Ask cancelled", "request_id", req.ID)
		return AnswersResponse{RequestID: req.ID}, ctx.Err()
	// Otherwise, wait for the user to answer the Question(s).
	case resp := <-ch:
		return resp, nil
	}
}

func (s *service) Answer(res AnswersResponse) {
	if ch, found := s.pendingRequests.Get(res.RequestID); found {
		slog.Debug("Answer", "request_id", res.RequestID, "answers", len(res.Answers))
		ch <- res
	} else {
		slog.Warn("Received answers for unknown questions", "request_id", res.RequestID)
	}
}
