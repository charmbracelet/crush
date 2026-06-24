package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/question"
)

const QuestionToolName = "question"

//go:embed question.md
var questionDescription string

// QuestionParams defines the parameters for the question tool.
type QuestionParams struct {
	Questions          []QuestionItem `json:"questions" description:"List of questions to present. Single item = no tabs, multiple = tabbed form."`
	ConfirmTitle       string         `json:"confirm_title,omitempty" description:"Title for the confirmation tab. Required for multi-question batches."`
	ConfirmDescription string         `json:"confirm_description,omitempty" description:"Description for the confirmation tab. Required for multi-question batches."`
}

// UnmarshalJSON handles models that double-serialize the questions field as a
// JSON string instead of a native array.
func (p *QuestionParams) UnmarshalJSON(data []byte) error {
	type Alias QuestionParams
	aux := &struct {
		Questions json.RawMessage `json:"questions"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if len(aux.Questions) == 0 {
		return nil
	}
	// Try array first.
	if err := json.Unmarshal(aux.Questions, &p.Questions); err != nil {
		// Fall back to string-encoded JSON array.
		var s string
		if err2 := json.Unmarshal(aux.Questions, &s); err2 != nil {
			return err
		}
		if err2 := json.Unmarshal([]byte(strings.TrimSpace(s)), &p.Questions); err2 != nil {
			return fmt.Errorf("questions must be an array: %w", err2)
		}
	}
	return nil
}

// QuestionItem is a single question from the tool input.
type QuestionItem struct {
	Label       string           `json:"label,omitempty" description:"Short tab header label (3 words max)."`
	Type        string           `json:"type" description:"The type of question: yes_no, single_choice, multi_choice, or free_text"`
	Question    string           `json:"question" description:"The question text"`
	Description string           `json:"description" description:"Required markdown description shown below the question"`
	Choices     []QuestionChoice `json:"choices,omitempty" description:"List of choices"`
	Options     []QuestionChoice `json:"options,omitempty"` // alias for Choices
}

// GetChoices returns choices, preferring the Choices field over Options.
func (q QuestionItem) GetChoices() []QuestionChoice {
	if len(q.Choices) > 0 {
		return q.Choices
	}
	return q.Options
}

// QuestionChoice represents a selectable option.
type QuestionChoice struct {
	ID          string `json:"id" description:"Unique identifier for this choice"`
	Label       string `json:"label" description:"Display text for this choice"`
	Description string `json:"description,omitempty" description:"Optional description for this choice"`
}

// NewQuestionTool creates a new question tool.
func NewQuestionTool(svc question.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		QuestionToolName,
		questionDescription,
		func(ctx context.Context, params QuestionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)

			if len(params.Questions) == 0 {
				return fantasy.NewTextErrorResponse("at least one question is required"), nil
			}
			if len(params.Questions) > question.MaxQuestions {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("exceeds maximum of %d questions per batch (got %d). Split into multiple batches and tell the user there will be follow-up questions", question.MaxQuestions, len(params.Questions))), nil
			}

			questions := make([]question.Question, len(params.Questions))
			for i, item := range params.Questions {
				qType := question.Type(item.Type)
				if qType != question.TypeYesNo && qType != question.TypeSingleChoice && qType != question.TypeMultiChoice && qType != question.TypeFreeText {
					label := item.Label
					if label == "" {
						label = item.Question
					}
					return fantasy.NewTextErrorResponse(fmt.Sprintf("question %d [%s]: invalid type %q (must be yes_no, single_choice, multi_choice, or free_text)", i+1, label, item.Type)), nil
				}
				questions[i] = question.Question{
					Type:        qType,
					Label:       item.Label,
					Text:        item.Question,
					Description: item.Description,
					Choices:     convertChoices(item.GetChoices()),
				}
			}

			req := question.Request{
				SessionID:          sessionID,
				ToolCallID:         call.ID,
				Questions:          questions,
				ConfirmTitle:       params.ConfirmTitle,
				ConfirmDescription: params.ConfirmDescription,
			}

			answers, err := svc.Ask(ctx, req)
			if err != nil {
				if errors.Is(err, question.ErrCancelled) {
					resp := fantasy.NewTextErrorResponse("User cancelled this question")
					resp.StopTurn = true
					return resp, nil
				}
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			return formatAnswers(answers, questions)
		},
	)
}

func convertChoices(in []QuestionChoice) []question.Choice {
	out := make([]question.Choice, len(in))
	for i, c := range in {
		out[i] = question.Choice{ID: c.ID, Label: c.Label, Description: c.Description}
	}
	return out
}

// formatAnswers converts answers into a tool response string for the LLM.
func formatAnswers(answers []question.Answer, questions []question.Question) (fantasy.ToolResponse, error) {
	var b strings.Builder
	for i, answer := range answers {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if i < len(questions) {
			fmt.Fprintf(&b, "Q%d: %s\n", i+1, questions[i].Text)
		}
		formatted, _ := formatAnswer(&answer, question.Type(""))
		b.WriteString(formatted.Content)
	}
	return fantasy.NewTextResponse(b.String()), nil
}

// formatAnswer formats a single answer for the LLM.
func formatAnswer(answer *question.Answer, _ question.Type) (fantasy.ToolResponse, error) {
	var b strings.Builder

	if answer.Yes != nil {
		if *answer.Yes {
			b.WriteString("User answered: yes")
		} else {
			b.WriteString("User answered: no")
		}
	} else if answer.FillInText != "" {
		fmt.Fprintf(&b, "User provided: %s", answer.FillInText)
	} else if len(answer.SelectedIDs) > 0 {
		data, _ := json.Marshal(answer.SelectedIDs)
		fmt.Fprintf(&b, "User selected: %s", string(data))
	} else {
		b.WriteString("User skipped this question")
	}

	if len(answer.Notes) > 0 {
		b.WriteString("\n\nNotes:")
		for key, note := range answer.Notes {
			if key == "_question" {
				fmt.Fprintf(&b, "\n- %s", note)
			} else {
				fmt.Fprintf(&b, "\n- [%s]: %s", key, note)
			}
		}
	}

	return fantasy.NewTextResponse(b.String()), nil
}
