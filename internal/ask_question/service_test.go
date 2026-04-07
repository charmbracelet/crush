package ask_question

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_AskAndAnswer(t *testing.T) {
	tests := []struct {
		name            string
		setupContext    func() (context.Context, context.CancelFunc)
		questionsReq    QuestionsRequest
		handleAnswering func(srv Service, req QuestionsRequest)
		expErr          error
		expAnswersRes   AnswersResponse
	}{
		{
			name: "successful single answer",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 2*time.Second)
			},
			questionsReq: NewQuestionsRequest("sess1", "tool1", []Question{
				{
					ID:          "q1",
					Question:    "Test?",
					Options:     []Option{{Label: "opt1"}},
					MultiSelect: false,
				},
			}),
			handleAnswering: func(srv Service, req QuestionsRequest) {
				srv.Answer(AnswersResponse{
					RequestID: req.ID,
					Answers:   []Answer{{ID: "q1", Selected: []string{"opt1"}}},
				})
			},
			expAnswersRes: AnswersResponse{
				Answers: []Answer{{ID: "q1", Selected: []string{"opt1"}}},
			},
		},
		{
			name: "successful multiple options, single select",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 2*time.Second)
			},
			questionsReq: NewQuestionsRequest("sess2", "tool2", []Question{
				{
					ID:       "q2",
					Question: "Choose one:",
					Options: []Option{
						{Label: "A"},
						{Label: "B"},
						{Label: "C"},
					},
					MultiSelect: false,
				},
			}),
			handleAnswering: func(srv Service, req QuestionsRequest) {
				srv.Answer(AnswersResponse{
					RequestID: req.ID,
					Answers:   []Answer{{ID: "q2", Selected: []string{"B"}}},
				})
			},
			expAnswersRes: AnswersResponse{
				Answers: []Answer{{ID: "q2", Selected: []string{"B"}}},
			},
		},
		{
			name: "successful multiple options, multi select",
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 2*time.Second)
			},
			questionsReq: NewQuestionsRequest("sess3", "tool3", []Question{
				{
					ID:       "q3",
					Question: "Choose many:",
					Options: []Option{
						{Label: "Apple"},
						{Label: "Banana"},
						{Label: "Cherry"},
					},
					MultiSelect: true,
				},
			}),
			handleAnswering: func(srv Service, req QuestionsRequest) {
				srv.Answer(AnswersResponse{
					RequestID: req.ID,
					Answers:   []Answer{{ID: "q3", Selected: []string{"Apple", "Cherry"}}},
				})
			},
			expAnswersRes: AnswersResponse{
				Answers: []Answer{{ID: "q3", Selected: []string{"Apple", "Cherry"}}},
			},
		},
		{
			name: "context canceled",
			setupContext: func() (context.Context, context.CancelFunc) {
				// Cancel immediately
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			questionsReq: NewQuestionsRequest("sess4", "tool4", []Question{
				{ID: "q4", Question: "Cancel me?"},
			}),
			handleAnswering: func(srv Service, req QuestionsRequest) {
				// Do nothing, let context cancellation abort Ask
			},
			expErr: context.Canceled,
			// When cancelled, it sets RequestID to the requested ID
			expAnswersRes: AnswersResponse{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewService()
			ctx, cancel := tt.setupContext()
			defer cancel()

			// Set the expected answers RequestID to the questions RequestID:
			// the latter is a dynamically generated UUID, so we inject it
			// into the expected answers response.
			tt.expAnswersRes.RequestID = tt.questionsReq.ID

			// Subscribe to wait for the Ask event so we know it's safe to Answer
			subCtx, subCancel := context.WithCancel(context.Background())
			defer subCancel()
			sub := srv.Subscribe(subCtx)

			// Answer the question in a separate goroutine
			go func() {
				select {
				case <-sub:
					tt.handleAnswering(srv, tt.questionsReq)
				case <-time.After(1 * time.Second):
					// Fallback to prevent hanging if Publish fails
				}
			}()

			// Ask the questions and check the answers
			resp, err := srv.Ask(ctx, tt.questionsReq)
			if tt.expErr != nil {
				require.ErrorIs(t, err, tt.expErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expAnswersRes, resp)

			// Verify cleanup
			s := srv.(*service)
			require.Equal(t, 0, s.pendingRequests.Len(), "pendingRequests should be empty after Ask returns")
		})
	}
}

func TestService_OrphanedAnswer(t *testing.T) {
	srv := NewService()
	// Answering an ID that doesn't exist should not panic or block
	require.NotPanics(t, func() {
		srv.Answer(AnswersResponse{RequestID: "fake-id"})
	})
}

func TestService_Concurrency(t *testing.T) {
	srv := NewService()
	var wg sync.WaitGroup

	numRequests := 50
	wg.Add(numRequests)

	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	sub := srv.Subscribe(subCtx)

	// Single goroutine to answer all questions as they come in via the pub/sub
	go func() {
		for event := range sub {
			srv.Answer(AnswersResponse{
				RequestID: event.Payload.ID,
				Answers:   []Answer{{ID: "q1", Selected: []string{"opt1"}}},
			})
		}
	}()

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			defer wg.Done()

			req := NewQuestionsRequest("sess1", "tool1", []Question{
				{ID: "q1", Question: "Test?"},
			})

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			resp, err := srv.Ask(ctx, req)
			require.NoError(t, err)
			require.Len(t, resp.Answers, 1)
			require.Equal(t, "opt1", resp.Answers[0].Selected[0])
		}(i)
	}

	wg.Wait()

	// Verify cleanup
	s := srv.(*service)
	require.Equal(t, 0, s.pendingRequests.Len(), "pendingRequests should be empty after all Asks return")
}
