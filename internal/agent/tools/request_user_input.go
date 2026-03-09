package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/userinput"
)

//go:embed request_user_input.md
var requestUserInputDescription []byte

const RequestUserInputToolName = "request_user_input"

type RequestUserInputParams struct {
	Questions []RequestUserInputQuestion `json:"questions" description:"One to three questions for the user to answer"`
}

type RequestUserInputQuestion struct {
	Header   string                   `json:"header" description:"Short section label shown in the UI"`
	ID       string                   `json:"id" description:"Stable identifier for the question"`
	Question string                   `json:"question" description:"Question text shown to the user"`
	Options  []RequestUserInputOption `json:"options" description:"Two or three mutually exclusive options"`
}

type RequestUserInputOption struct {
	Label       string `json:"label" description:"Short user-facing option label"`
	Description string `json:"description" description:"One sentence explaining the tradeoff"`
}

type RequestUserInputResult struct {
	Status       string                   `json:"status"`
	Answers      []RequestUserInputAnswer `json:"answers"`
	CancelReason string                   `json:"cancel_reason,omitempty"`
}

type RequestUserInputAnswer struct {
	QuestionID     string `json:"question_id"`
	SelectedOption string `json:"selected_option,omitempty"`
	CustomInput    string `json:"custom_input,omitempty"`
}

func NewRequestUserInputTool(service userinput.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RequestUserInputToolName,
		string(requestUserInputDescription),
		func(ctx context.Context, params RequestUserInputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for request_user_input")
			}
			if len(params.Questions) == 0 || len(params.Questions) > 3 {
				return fantasy.NewTextErrorResponse("questions must contain between 1 and 3 items"), nil
			}

			questions := make([]userinput.Question, 0, len(params.Questions))
			for _, question := range params.Questions {
				if len(question.Options) < 2 || len(question.Options) > 3 {
					return fantasy.NewTextErrorResponse("each question must have between 2 and 3 options"), nil
				}
				options := make([]userinput.Option, 0, len(question.Options))
				for _, option := range question.Options {
					options = append(options, userinput.Option{
						Label:       option.Label,
						Description: option.Description,
					})
				}
				questions = append(questions, userinput.Question{
					Header:   question.Header,
					ID:       question.ID,
					Question: question.Question,
					Options:  options,
				})
			}

			requestCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			response, err := service.Request(requestCtx, userinput.CreateRequest{
				SessionID:  sessionID,
				ToolCallID: call.ID,
				Questions:  questions,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			result := RequestUserInputResult{
				Status:       string(response.Status),
				CancelReason: response.CancelReason,
				Answers:      make([]RequestUserInputAnswer, 0, len(response.Answers)),
			}
			for _, answer := range response.Answers {
				result.Answers = append(result.Answers, RequestUserInputAnswer{
					QuestionID:     answer.QuestionID,
					SelectedOption: answer.SelectedOption,
					CustomInput:    answer.CustomInput,
				})
			}

			payload, err := json.Marshal(result)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("marshal user input result: %w", err)
			}
			return fantasy.NewTextResponse(string(payload)), nil
		},
	)
}
