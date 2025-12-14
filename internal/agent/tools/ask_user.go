package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/askuser"
)

//go:embed ask_user.md
var askUserDescription []byte

const AskUserToolName = "ask_user"

// AskUserParams defines the tool parameters.
type AskUserParams struct {
	Questions []AskUserQuestion `json:"questions" description:"1-4 questions to ask the user"`
}

// AskUserQuestion represents a single question.
type AskUserQuestion struct {
	Question    string          `json:"question" description:"The question to ask the user"`
	Header      string          `json:"header" description:"Short label for the question (max 12 chars)"`
	Options     []AskUserOption `json:"options" description:"2-4 answer choices"`
	MultiSelect bool            `json:"multi_select,omitempty" description:"Allow multiple selections if true"`
}

// AskUserOption represents an answer option.
type AskUserOption struct {
	Label       string `json:"label" description:"Short label for the option (1-5 words)"`
	Description string `json:"description,omitempty" description:"Explanation of what this option means"`
}

// AskUserResponseMetadata for tool response.
type AskUserResponseMetadata struct {
	Questions []AskUserQuestion `json:"questions"`
	Answers   []askuser.Answer  `json:"answers"`
	Cancelled bool              `json:"cancelled"`
}

var (
	ErrAskUserCancelled   = errors.New("user cancelled the question")
	ErrTooManyQuestions   = errors.New("maximum 4 questions allowed per call")
	ErrTooFewQuestions    = errors.New("at least 1 question is required")
	ErrInvalidOptionCount = errors.New("each question must have 2-4 options")
	ErrHeaderTooLong      = errors.New("header must be 12 characters or less")
	ErrEmptyQuestion      = errors.New("question text cannot be empty")
	ErrEmptyOptionLabel   = errors.New("option label cannot be empty")
)

// validateAskUserParams validates the parameters.
func validateAskUserParams(params AskUserParams) error {
	if len(params.Questions) == 0 {
		return ErrTooFewQuestions
	}
	if len(params.Questions) > 4 {
		return ErrTooManyQuestions
	}

	for i, q := range params.Questions {
		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("question %d: %w", i+1, ErrEmptyQuestion)
		}
		if len(q.Header) > 12 {
			return fmt.Errorf("question %d: %w", i+1, ErrHeaderTooLong)
		}
		if len(q.Options) < 2 || len(q.Options) > 4 {
			return fmt.Errorf("question %d: %w", i+1, ErrInvalidOptionCount)
		}
		for j, opt := range q.Options {
			if strings.TrimSpace(opt.Label) == "" {
				return fmt.Errorf("question %d, option %d: %w", i+1, j+1, ErrEmptyOptionLabel)
			}
		}
	}

	return nil
}

// NewAskUserTool creates a new ask_user tool.
func NewAskUserTool(askUserService askuser.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AskUserToolName,
		string(askUserDescription),
		func(ctx context.Context, params AskUserParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Validate params
			if err := validateAskUserParams(params); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required")
			}

			// Convert to service types
			questions := make([]askuser.Question, len(params.Questions))
			for i, q := range params.Questions {
				options := make([]askuser.QuestionOption, len(q.Options))
				for j, opt := range q.Options {
					options[j] = askuser.QuestionOption{
						Label:       opt.Label,
						Description: opt.Description,
					}
				}
				questions[i] = askuser.Question{
					Question:    q.Question,
					Header:      q.Header,
					Options:     options,
					MultiSelect: q.MultiSelect,
				}
			}

			// Make blocking request
			response, err := askUserService.Request(askuser.CreateAskUserRequest{
				SessionID:  sessionID,
				ToolCallID: call.ID,
				Questions:  questions,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			if response.Cancelled {
				return fantasy.ToolResponse{}, ErrAskUserCancelled
			}

			// Format response text
			var responseText strings.Builder
			responseText.WriteString("User responses:\n\n")

			for i, answer := range response.Answers {
				q := params.Questions[i]
				responseText.WriteString(fmt.Sprintf("**%s**: %s\n", q.Header, q.Question))

				if answer.IsOther {
					responseText.WriteString(fmt.Sprintf("Answer: %s (custom response)\n\n", answer.OtherText))
				} else if q.MultiSelect {
					responseText.WriteString("Selected: ")
					labels := make([]string, 0, len(answer.SelectedIndices))
					for _, idx := range answer.SelectedIndices {
						if idx >= 0 && idx < len(q.Options) {
							labels = append(labels, q.Options[idx].Label)
						}
					}
					responseText.WriteString(strings.Join(labels, ", "))
					responseText.WriteString("\n\n")
				} else {
					if answer.SelectedIndex >= 0 && answer.SelectedIndex < len(q.Options) {
						responseText.WriteString(fmt.Sprintf("Answer: %s\n\n", q.Options[answer.SelectedIndex].Label))
					}
				}
			}

			metadata := AskUserResponseMetadata{
				Questions: params.Questions,
				Answers:   response.Answers,
				Cancelled: false,
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(responseText.String()),
				metadata,
			), nil
		})
}
