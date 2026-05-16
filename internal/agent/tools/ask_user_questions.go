package tools

import (
	"context"
	_ "embed"
	"encoding/json"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/questions"
)

const askUserQuestionsToolName = "ask_user_questions"

//go:embed ask_user_questions.md
var askUserQuestionDescription []byte

// AskUserQuestionParams holds the parameters for the ask_user_questions tool.
type AskUserQuestionParams struct {
	Questions []questions.Question `json:"questions" description:"Array of questions to ask the user"`
}

// NewAskUserQuestionsTool creates a tool that asks the user multiple-choice questions.
func NewAskUserQuestionsTool(questionsService questions.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		askUserQuestionsToolName,
		string(askUserQuestionDescription),
		func(ctx context.Context, params AskUserQuestionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			req := questions.NewQuestionsRequest(sessionID, call.ID, params.Questions)

			resp, err := questionsService.Ask(ctx, req)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			jsonResp, err := json.Marshal(resp.Answers)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			return fantasy.NewTextResponse(string(jsonResp)), nil
		},
	)
}
