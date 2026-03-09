package userinput

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceRequestResolve(t *testing.T) {
	t.Parallel()

	svc := NewService()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	responses := make(chan Response, 1)
	go func() {
		resp, err := svc.Request(ctx, CreateRequest{
			SessionID:  "session-1",
			ToolCallID: "call-1",
			Questions: []Question{{
				Header:   "Mode",
				ID:       "mode",
				Question: "Choose",
				Options:  []Option{{Label: "A", Description: "Option A"}},
			}},
		})
		require.NoError(t, err)
		responses <- resp
	}()

	requestEvent := <-svc.Subscribe(ctx)
	req := requestEvent.Payload
	svc.Resolve(Response{
		RequestID:  req.ID,
		SessionID:  req.SessionID,
		ToolCallID: req.ToolCallID,
		Status:     ResponseStatusSubmitted,
		Answers: []Answer{{
			QuestionID:     "mode",
			SelectedOption: "A",
		}},
	})

	resp := <-responses
	require.Equal(t, ResponseStatusSubmitted, resp.Status)
	require.Len(t, resp.Answers, 1)
}

func TestServiceRequestTimeout(t *testing.T) {
	t.Parallel()

	svc := NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	resp, err := svc.Request(ctx, CreateRequest{
		SessionID:  "session-1",
		ToolCallID: "call-1",
		Questions:  []Question{{Header: "Mode", ID: "mode", Question: "Choose", Options: []Option{{Label: "A", Description: "Option A"}}}},
	})
	require.NoError(t, err)
	require.Equal(t, ResponseStatusTimedOut, resp.Status)
}

func TestServiceAllowsConcurrentRequests(t *testing.T) {
	t.Parallel()

	svc := NewService()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	responses := make(chan Response, 2)
	for i := range 2 {
		go func(index int) {
			resp, err := svc.Request(ctx, CreateRequest{
				SessionID:  "session-1",
				ToolCallID: "call-" + string(rune('1'+index)),
				Questions: []Question{{
					Header:   "Mode",
					ID:       "mode",
					Question: "Choose",
					Options: []Option{
						{Label: "A", Description: "Option A"},
						{Label: "B", Description: "Option B"},
					},
				}},
			})
			require.NoError(t, err)
			responses <- resp
		}(i)
	}

	requests := make([]Request, 0, 2)
	sub := svc.Subscribe(ctx)
	for range 2 {
		requests = append(requests, (<-sub).Payload)
	}

	for _, req := range requests {
		svc.Resolve(Response{
			RequestID:  req.ID,
			SessionID:  req.SessionID,
			ToolCallID: req.ToolCallID,
			Status:     ResponseStatusSubmitted,
			Answers: []Answer{{
				QuestionID:     "mode",
				SelectedOption: "A",
			}},
		})
	}

	for range 2 {
		resp := <-responses
		require.Equal(t, ResponseStatusSubmitted, resp.Status)
	}
}
