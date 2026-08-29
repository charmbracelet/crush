package tools

import (
	"context"
	_ "embed"
	"errors"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/question"
)

// PresentPlanToolName is the tool the model calls in plan mode to hand its
// finished plan to the user for approval.
const PresentPlanToolName = "present_plan"

//go:embed present_plan.md
var presentPlanDescription string

// PresentPlanParams defines the parameters for the present_plan tool.
type PresentPlanParams struct {
	// Plan is the full markdown plan the model wants the user to approve.
	Plan string `json:"plan" description:"The complete plan (markdown) you want the user to approve before you start implementing."`
}

// PlanApprovalOption identifies the user's decision on a presented plan.
type PlanApprovalOption string

const (
	// PlanApprovalExecute approves the plan and exits plan mode to start
	// implementing.
	PlanApprovalExecute PlanApprovalOption = "execute"
	// PlanApprovalContinue keeps plan mode active so the plan can be
	// refined further.
	PlanApprovalContinue PlanApprovalOption = "continue"
	// PlanApprovalCancel dismisses the plan without executing.
	PlanApprovalCancel PlanApprovalOption = "cancel"
)

// PlanApprovalQuestionID is the question ID used for the plan approval
// single-choice question published through the question service. The UI
// special-cases this question and renders a plan-approval dialog.
const PlanApprovalQuestionID = "_plan_approval"

// PlanApprovalBatchID is the batch ID used for plan approval requests.
const PlanApprovalBatchID = "_plan_approval_batch"

// NewPresentPlanTool creates the present_plan tool. It publishes a
// plan-approval question through the question service and blocks until the
// user picks an option. The returned tool response tells the model whether
// to start implementing or keep refining.
func NewPresentPlanTool(svc question.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		PresentPlanToolName,
		presentPlanDescription,
		func(ctx context.Context, params PresentPlanParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Plan) == "" {
				return fantasy.NewTextErrorResponse("plan is required and must not be empty"), nil
			}

			sessionID := GetSessionFromContext(ctx)

			req := question.Request{
				ID:                 PlanApprovalBatchID,
				SessionID:          sessionID,
				ToolCallID:         call.ID,
				ConfirmTitle:       "Approve this plan?",
				ConfirmDescription: "The plan below will be executed once approved.",
				Questions: []question.Question{
					{
						ID:          PlanApprovalQuestionID,
						Type:        question.TypeSingleChoice,
						Label:       "Plan",
						Text:        params.Plan,
						Description: "Approve this plan to start implementing, or keep refining it.",
						Choices: []question.Choice{
							{
								ID:    string(PlanApprovalExecute),
								Label: "Execute",
								Description: "Approve and exit plan mode to implement.",
							},
							{
								ID:    string(PlanApprovalContinue),
								Label: "Continue planning",
								Description: "Stay in plan mode and refine the plan.",
							},
							{
								ID:    string(PlanApprovalCancel),
								Label: "Cancel",
								Description: "Dismiss the plan without executing.",
							},
						},
					},
				},
			}

			answers, err := svc.Ask(ctx, req)
			if err != nil {
				if errors.Is(err, question.ErrCancelled) {
					resp := fantasy.NewTextErrorResponse("Plan review was cancelled. Continue investigating and refine the plan, then call present_plan again when it is complete.")
					resp.StopTurn = true
					return resp, nil
				}
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			return formatPlanApproval(answers)
		},
	)
}

// formatPlanApproval converts the plan-approval answer into the tool
// response fed back to the model.
func formatPlanApproval(answers []question.Answer) (fantasy.ToolResponse, error) {
	option := PlanApprovalCancel
	if len(answers) > 0 {
		if len(answers[0].SelectedIDs) > 0 {
			option = PlanApprovalOption(answers[0].SelectedIDs[0])
		} else if answers[0].FillInText != "" {
			option = PlanApprovalOption(strings.TrimSpace(strings.ToLower(answers[0].FillInText)))
		}
	}

	switch option {
	case PlanApprovalExecute:
		resp := fantasy.NewTextResponse("User approved the plan. You can now start implementing. Start with updating your todo list if applicable.")
		resp.StopTurn = true
		return resp, nil
	case PlanApprovalContinue:
		return fantasy.NewTextResponse("User wants to keep refining the plan. Stay in plan mode, address their feedback, and call present_plan again once the plan is complete."), nil
	default:
		return fantasy.NewTextResponse("User dismissed the plan without approving it. Stay in plan mode, refine the plan, and call present_plan again when it is ready."), nil
	}
}

// IsPlanApprovalBatch reports whether a question batch is a plan-approval
// request (used by the UI to render the plan dialog).
func IsPlanApprovalBatch(batch question.Request) bool {
	return batch.ID == PlanApprovalBatchID
}
