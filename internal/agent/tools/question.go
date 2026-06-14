package tools

import (
	"context"
	_ "embed"
	"encoding/json"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/question"
)

const QuestionToolName = "question"

//go:embed question.md
var questionDescription string

// QuestionParams defines the parameters the model passes when calling this tool.
type QuestionParams struct {
	Question      string   `json:"question" description:"The clarifying question to ask the user"`
	Options       []string `json:"options,omitempty" description:"Up to 4 suggested answers; the user may also type a custom answer"`
	AllowMultiple bool     `json:"allow_multiple,omitempty" description:"Whether the user may select multiple options. Returns a JSON array of selected and custom answer strings"`
}

func NewQuestionTool(svc question.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		QuestionToolName,
		questionDescription,
		func(ctx context.Context, params QuestionParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Question == "" {
				return fantasy.NewTextErrorResponse("question parameter is required"), nil
			}
			response, err := svc.Ask(ctx, params.Question, params.Options, params.AllowMultiple)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if params.AllowMultiple {
				if len(response.Answers) == 0 {
					return fantasy.NewTextErrorResponse("the user dismissed the question without answering"), nil
				}
				answerJSON, err := json.Marshal(response.Answers)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(string(answerJSON)), nil
			}
			if len(response.Answers) == 0 || response.Answers[0] == "" {
				return fantasy.NewTextErrorResponse("the user dismissed the question without answering"), nil
			}
			return fantasy.NewTextResponse(response.Answers[0]), nil
		},
	)
}
