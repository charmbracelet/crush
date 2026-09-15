package question

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testRequest() Request {
	return Request{
		Questions: []Question{{
			Type:        TypeSingleChoice,
			Text:        "Pick one",
			Description: "A choice is needed.",
			Choices: []Choice{
				{ID: "a", Label: "A"},
				{ID: "b", Label: "B"},
			},
		}},
	}
}

func TestAskFailsFastWhilePending(t *testing.T) {
	t.Parallel()
	svc := NewService()
	ctx := context.Background()

	first := make(chan error, 1)
	go func() {
		_, err := svc.Ask(ctx, testRequest())
		first <- err
	}()

	// Wait for the first Ask to install its pending question.
	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.pending != nil
	}, time.Second, time.Millisecond)

	// A concurrent Ask must fail fast, not orphan its channels.
	_, err := svc.Ask(ctx, testRequest())
	require.ErrorIs(t, err, ErrQuestionPending)

	// Resolving frees the slot for the next Ask.
	require.True(t, svc.Answer([]Answer{{SelectedIDs: []string{"a"}}}))
	require.NoError(t, <-first)

	second := make(chan error, 1)
	go func() {
		_, err := svc.Ask(ctx, testRequest())
		second <- err
	}()
	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.pending != nil
	}, time.Second, time.Millisecond)
	require.True(t, svc.Answer([]Answer{{SelectedIDs: []string{"b"}}}))
	require.NoError(t, <-second)
}
