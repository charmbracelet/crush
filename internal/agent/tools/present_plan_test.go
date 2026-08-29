package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
)

// stubQuestionService is a minimal question.Service that records the
// published request and returns canned answers.
type stubQuestionService struct {
	req     *question.Request
	answers []question.Answer
}

func (s *stubQuestionService) Ask(ctx context.Context, req question.Request) ([]question.Answer, error) {
	s.req = &req
	return s.answers, nil
}

func (s *stubQuestionService) Answer([]question.Answer) bool { return false }
func (s *stubQuestionService) Cancel() bool                  { return false }
func (s *stubQuestionService) Subscribe(ctx context.Context) <-chan pubsub.Event[question.Request] {
	return make(<-chan pubsub.Event[question.Request])
}
func (s *stubQuestionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[question.Notification] {
	return make(<-chan pubsub.Event[question.Notification])
}

func TestPresentPlanTool_PublishesApprovalQuestion(t *testing.T) {
	svc := &stubQuestionService{
		answers: []question.Answer{{
			QuestionID:  PlanApprovalQuestionID,
			SelectedIDs: []string{string(PlanApprovalExecute)},
		}},
	}
	tool := NewPresentPlanTool(svc)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")

	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: "present_plan", Input: `{"plan":"Step 1: read files\nStep 2: edit"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}
	if svc.req == nil {
		t.Fatal("expected question request to be published")
	}
	if svc.req.ID != PlanApprovalBatchID {
		t.Fatalf("expected plan approval batch ID, got %q", svc.req.ID)
	}
	if len(svc.req.Questions) != 1 || svc.req.Questions[0].ID != PlanApprovalQuestionID {
		t.Fatalf("expected single plan approval question")
	}
	if len(svc.req.Questions[0].Choices) != 3 {
		t.Fatalf("expected 3 approval choices, got %d", len(svc.req.Questions[0].Choices))
	}
	if !resp.StopTurn {
		t.Error("expected StopTurn after approval so the model starts implementing")
	}
	if !IsPlanApprovalBatch(*svc.req) {
		t.Error("IsPlanApprovalBatch should recognize the plan approval batch")
	}
}

func TestPresentPlanTool_EmptyPlanRejected(t *testing.T) {
	svc := &stubQuestionService{}
	tool := NewPresentPlanTool(svc)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")

	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: "present_plan", Input: `{"plan":"   "}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Fatal("expected empty plan to be rejected")
	}
	if svc.req != nil {
		t.Fatal("expected no question request for empty plan")
	}
}

func TestPresentPlanTool_ContinueKeepsPlanMode(t *testing.T) {
	svc := &stubQuestionService{
		answers: []question.Answer{{
			QuestionID:  PlanApprovalQuestionID,
			SelectedIDs: []string{string(PlanApprovalContinue)},
		}},
	}
	tool := NewPresentPlanTool(svc)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")

	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: "present_plan", Input: `{"plan":"plan"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}
	if resp.StopTurn {
		t.Error("continue should not stop the turn")
	}
}
