package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/questions"
	"github.com/stretchr/testify/require"
)

type mockQuestionsService struct {
	*pubsub.Broker[questions.QuestionsRequest]
	response questions.QuestionsResponse
	err      error
}

func (m *mockQuestionsService) Ask(_ context.Context, _ questions.QuestionsRequest) (questions.QuestionsResponse, error) {
	if m.err != nil {
		return questions.QuestionsResponse{}, m.err
	}
	return m.response, nil
}

func (m *mockQuestionsService) Answer(_ questions.QuestionsResponse) {
}

func TestAskUserQuestionsTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		questions     []questions.Question
		mockResponse  questions.QuestionsResponse
		mockErr       error
		expectedError string
	}{
		{
			name: "single question with single answer",
			questions: []questions.Question{
				{
					ID:       "q1",
					Question: "Do you like Go?",
					Options: []questions.Option{
						{Label: "Yes"},
						{Label: "No"},
					},
				},
			},
			mockResponse: questions.QuestionsResponse{
				RequestID: "req-1",
				Answers: []questions.Answer{
					{
						ID:       "q1",
						Selected: []string{"Yes"},
					},
				},
			},
		},
		{
			name: "single question with multiple answers",
			questions: []questions.Question{
				{
					ID:          "q2",
					Question:    "Which languages do you know?",
					MultiSelect: true,
					Options: []questions.Option{
						{Label: "Go"},
						{Label: "Python"},
						{Label: "Rust"},
					},
				},
			},
			mockResponse: questions.QuestionsResponse{
				RequestID: "req-2",
				Answers: []questions.Answer{
					{
						ID:       "q2",
						Selected: []string{"Go", "Rust"},
					},
				},
			},
		},
		{
			name: "multiple questions with mixed answers",
			questions: []questions.Question{
				{
					ID:       "q3",
					Question: "Favorite editor?",
					Options: []questions.Option{
						{Label: "Vim"},
						{Label: "Emacs"},
						{Label: "VSCode"},
					},
				},
				{
					ID:          "q4",
					Question:    "Preferred OS?",
					MultiSelect: true,
					Options: []questions.Option{
						{Label: "Linux"},
						{Label: "macOS"},
						{Label: "Windows"},
					},
				},
			},
			mockResponse: questions.QuestionsResponse{
				RequestID: "req-3",
				Answers: []questions.Answer{
					{
						ID:       "q3",
						Selected: []string{"Vim"},
					},
					{
						ID:       "q4",
						Selected: []string{"Linux", "macOS"},
					},
				},
			},
		},
		{
			name: "service returns error",
			questions: []questions.Question{
				{
					ID:       "q5",
					Question: "Trigger error?",
					Options:  []questions.Option{{Label: "Yes"}},
				},
			},
			mockErr:       errors.New("service failure"),
			expectedError: "service failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockService := &mockQuestionsService{
				Broker:   pubsub.NewBroker[questions.QuestionsRequest](),
				response: tt.mockResponse,
				err:      tt.mockErr,
			}

			tool := NewAskUserQuestionsTool(mockService)
			ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

			params := AskUserQuestionParams{
				Questions: tt.questions,
			}
			input, err := json.Marshal(params)
			require.NoError(t, err)

			call := fantasy.ToolCall{
				ID:    "test-call",
				Name:  askUserQuestionsToolName,
				Input: string(input),
			}

			resp, err := tool.Run(ctx, call)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.False(t, resp.IsError)

			var gotAnswers []questions.Answer
			require.NoError(t, json.Unmarshal([]byte(resp.Content), &gotAnswers))
			require.Equal(t, tt.mockResponse.Answers, gotAnswers)
		})
	}
}
