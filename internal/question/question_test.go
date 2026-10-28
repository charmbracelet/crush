package question

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freeTextRequest(sessionID string) Request {
	return Request{
		SessionID: sessionID,
		Questions: []Question{{
			Type:        TypeFreeText,
			Text:        "What should we do?",
			Description: "Share your thoughts",
		}},
	}
}

// askResult is the outcome of an Ask call.
type askResult struct {
	answers []Answer
	err     error
}

// askAsync asks batch id (one free-text question with ID "q-<id>") in
// session "s-<id>" and returns once the batch is pending. The Ask's
// outcome arrives on the returned channel.
func askAsync(t *testing.T, svc *questionService, id string) <-chan askResult {
	t.Helper()
	res := make(chan askResult, 1)
	go func() {
		req := freeTextRequest("s-" + id)
		req.ID = id
		req.Questions[0].ID = "q-" + id
		answers, err := svc.Ask(t.Context(), req)
		res <- askResult{answers: answers, err: err}
	}()
	require.Eventually(t, func() bool {
		return slices.ContainsFunc(svc.Pending(), func(r Request) bool { return r.ID == id })
	}, 2*time.Second, 5*time.Millisecond, "batch %s must be pending", id)
	return res
}

// TestPending verifies that Pending reports the in-flight request
// (including its session) only while it awaits an answer.
func TestPending(t *testing.T) {
	t.Parallel()
	svc := NewService()

	assert.Empty(t, svc.Pending(), "nothing pending yet")

	done := make(chan []Answer, 1)
	go func() {
		answers, _ := svc.Ask(t.Context(), freeTextRequest("s-ask"))
		done <- answers
	}()

	var pending []Request
	require.Eventually(t, func() bool {
		pending = svc.Pending()
		return len(pending) == 1
	}, 2*time.Second, 5*time.Millisecond, "question must be pending")
	require.Equal(t, "s-ask", pending[0].SessionID)
	require.NotEmpty(t, pending[0].ID)
	require.Len(t, pending[0].Questions, 1)

	require.True(t, svc.Answer(pending[0].ID, []Answer{{QuestionID: pending[0].Questions[0].ID, FillInText: "ship it"}}))
	answers := <-done
	require.Len(t, answers, 1)
	assert.Equal(t, "ship it", answers[0].FillInText)

	assert.Empty(t, svc.Pending(), "answered question must no longer be pending")
}

// TestAnswerNotificationCarriesSessionID verifies that the resolution
// notification names the session the question belongs to, so SSE
// routing can scope it to that session's viewers.
func TestAnswerNotificationCarriesSessionID(t *testing.T) {
	t.Parallel()
	svc := NewService()

	notifications := svc.SubscribeNotifications(t.Context())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = svc.Ask(t.Context(), freeTextRequest("s-answer"))
	}()

	var pending Request
	require.Eventually(t, func() bool {
		reqs := svc.Pending()
		if len(reqs) == 0 {
			return false
		}
		pending = reqs[0]
		return true
	}, 2*time.Second, 5*time.Millisecond)

	require.True(t, svc.Answer(pending.ID, []Answer{{QuestionID: pending.Questions[0].ID, FillInText: "yes"}}))
	<-done

	ev := <-notifications
	assert.Equal(t, pending.ID, ev.Payload.BatchID)
	assert.Equal(t, "s-answer", ev.Payload.SessionID)
}

// TestCancelNotificationCarriesSessionID covers the cancel path of
// the resolution notification.
func TestCancelNotificationCarriesSessionID(t *testing.T) {
	t.Parallel()
	svc := NewService()

	notifications := svc.SubscribeNotifications(t.Context())

	done := make(chan error, 1)
	go func() {
		_, err := svc.Ask(t.Context(), freeTextRequest("s-cancel"))
		done <- err
	}()

	var pending Request
	require.Eventually(t, func() bool {
		reqs := svc.Pending()
		if len(reqs) == 0 {
			return false
		}
		pending = reqs[0]
		return true
	}, 2*time.Second, 5*time.Millisecond)

	require.True(t, svc.Cancel(pending.ID))
	require.ErrorIs(t, <-done, ErrCancelled)

	ev := <-notifications
	assert.Equal(t, pending.ID, ev.Payload.BatchID)
	assert.Equal(t, "s-cancel", ev.Payload.SessionID)
}

// TestAsk_ConcurrentBatchesAreRoutedByID verifies that batches pending
// in two sessions at once are resolved independently, each by the
// answer or cancellation that names it.
func TestAsk_ConcurrentBatchesAreRoutedByID(t *testing.T) {
	t.Parallel()
	svc := NewService()

	resA := askAsync(t, svc, "A")
	resB := askAsync(t, svc, "B")

	require.True(t, svc.Answer("A", []Answer{{QuestionID: "q-A", FillInText: "for A"}}))
	got := <-resA
	require.NoError(t, got.err)
	require.Len(t, got.answers, 1)
	require.Equal(t, "for A", got.answers[0].FillInText)

	pending := svc.Pending()
	require.Len(t, pending, 1, "batch B must still be pending")
	require.Equal(t, "B", pending[0].ID)
	require.Equal(t, "s-B", pending[0].SessionID)

	require.False(t, svc.Answer("A", nil), "batch A is already resolved")
	require.True(t, svc.Cancel("B"))
	require.ErrorIs(t, (<-resB).err, ErrCancelled)
	require.Empty(t, svc.Pending())
}

// TestResolve_ExactlyOneWinner verifies that when answers and
// cancellations race for one batch, exactly one of them resolves it.
func TestResolve_ExactlyOneWinner(t *testing.T) {
	t.Parallel()
	svc := NewService()

	res := askAsync(t, svc, "A")

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			var won bool
			if i%2 == 0 {
				won = svc.Answer("A", []Answer{{QuestionID: "q-A"}})
			} else {
				won = svc.Cancel("A")
			}
			if won {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 1, wins.Load())
	<-res
}

// TestEmptyBatchID verifies that callers that predate batch IDs still
// resolve the only pending batch, and resolve nothing rather than
// guess when several are pending.
func TestEmptyBatchID(t *testing.T) {
	t.Parallel()
	svc := NewService()

	resA := askAsync(t, svc, "A")
	resB := askAsync(t, svc, "B")

	require.False(t, svc.Answer("", nil), "ambiguous with two batches pending")
	require.False(t, svc.Cancel(""), "ambiguous with two batches pending")
	require.Len(t, svc.Pending(), 2)

	require.True(t, svc.Cancel("A"))
	require.ErrorIs(t, (<-resA).err, ErrCancelled)

	require.True(t, svc.Answer("", []Answer{{QuestionID: "q-B", FillInText: "for B"}}))
	got := <-resB
	require.NoError(t, got.err)
	require.Equal(t, "for B", got.answers[0].FillInText)
}

// TestAsk_RejectsDuplicateBatchID verifies that a batch cannot take
// over the ID of one that is still pending.
func TestAsk_RejectsDuplicateBatchID(t *testing.T) {
	t.Parallel()
	svc := NewService()

	res := askAsync(t, svc, "A")

	dup := freeTextRequest("s-other")
	dup.ID = "A"
	_, err := svc.Ask(t.Context(), dup)
	require.Error(t, err)

	require.True(t, svc.Answer("A", []Answer{{QuestionID: "q-A", FillInText: "for A"}}))
	got := <-res
	require.NoError(t, got.err)
	require.Equal(t, "for A", got.answers[0].FillInText)
}

// TestAsk_AbandonedBatchIsDropped verifies that a batch whose Ask gave
// up is no longer pending, so a late answer is refused rather than
// sent to nobody.
func TestAsk_AbandonedBatchIsDropped(t *testing.T) {
	t.Parallel()
	svc := NewService()

	ctx, cancel := context.WithCancel(t.Context())
	req := freeTextRequest("s-gone")
	req.ID = "A"
	done := make(chan error, 1)
	go func() {
		_, err := svc.Ask(ctx, req)
		done <- err
	}()
	require.Eventually(t, func() bool {
		return len(svc.Pending()) == 1
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Empty(t, svc.Pending())
	require.False(t, svc.Answer("A", nil))
	require.False(t, svc.Cancel("A"))
}
