package question

import (
	"context"
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

// TestPending verifies that Pending reports the in-flight request
// (including its session) only while it awaits an answer.
func TestPending(t *testing.T) {
	t.Parallel()
	svc := NewService()

	_, ok := svc.Pending()
	assert.False(t, ok, "nothing pending yet")

	done := make(chan []Answer, 1)
	go func() {
		answers, _ := svc.Ask(t.Context(), freeTextRequest("s-ask"))
		done <- answers
	}()

	var pending Request
	require.Eventually(t, func() bool {
		pending, ok = svc.Pending()
		return ok
	}, 2*time.Second, 5*time.Millisecond, "question must be pending")
	require.Equal(t, "s-ask", pending.SessionID)
	require.NotEmpty(t, pending.ID)
	require.Len(t, pending.Questions, 1)

	require.True(t, svc.Answer([]Answer{{QuestionID: pending.Questions[0].ID, FillInText: "ship it"}}))
	answers := <-done
	require.Len(t, answers, 1)
	assert.Equal(t, "ship it", answers[0].FillInText)

	_, ok = svc.Pending()
	assert.False(t, ok, "answered question must no longer be pending")
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
		var ok bool
		pending, ok = svc.Pending()
		return ok
	}, 2*time.Second, 5*time.Millisecond)

	require.True(t, svc.Answer([]Answer{{QuestionID: pending.Questions[0].ID, FillInText: "yes"}}))
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
		var ok bool
		pending, ok = svc.Pending()
		return ok
	}, 2*time.Second, 5*time.Millisecond)

	require.True(t, svc.Cancel())
	require.ErrorIs(t, <-done, ErrCancelled)

	ev := <-notifications
	assert.Equal(t, pending.ID, ev.Payload.BatchID)
	assert.Equal(t, "s-cancel", ev.Payload.SessionID)
}

type askResult struct {
	answers []Answer
	err     error
}

func askAsync(ctx context.Context, svc *questionService, sessionID string) <-chan askResult {
	ch := make(chan askResult, 1)
	go func() {
		answers, err := svc.Ask(ctx, freeTextRequest(sessionID))
		ch <- askResult{answers, err}
	}()
	return ch
}

func recvEvent[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timed out waiting")
		panic("unreachable")
	}
}

// TestAskFromTwoSessionsBothAnswered verifies that a second Ask from
// another session queues behind the first instead of overwriting it.
func TestAskFromTwoSessionsBothAnswered(t *testing.T) {
	t.Parallel()
	svc := NewService()
	events := svc.Subscribe(t.Context())

	resA := askAsync(t.Context(), svc, "a")
	reqA := recvEvent(t, events).Payload
	require.Equal(t, "a", reqA.SessionID)

	resB := askAsync(t.Context(), svc, "b")
	// Give B time to enter Ask; without queuing it would overwrite A here.
	time.Sleep(100 * time.Millisecond)

	ansA := []Answer{{QuestionID: reqA.Questions[0].ID, FillInText: "answer-a"}}
	require.True(t, svc.Answer(ansA))
	require.Equal(t, ansA, recvEvent(t, resA).answers)

	reqB := recvEvent(t, events).Payload
	require.Equal(t, "b", reqB.SessionID)
	ansB := []Answer{{QuestionID: reqB.Questions[0].ID, FillInText: "answer-b"}}
	require.True(t, svc.Answer(ansB))
	require.Equal(t, ansB, recvEvent(t, resB).answers)
}

// TestAskQueuedCancelledByContext verifies that an Ask waiting in the
// queue returns as soon as its own context is cancelled.
func TestAskQueuedCancelledByContext(t *testing.T) {
	t.Parallel()
	svc := NewService()
	events := svc.Subscribe(t.Context())

	resA := askAsync(t.Context(), svc, "a")
	reqA := recvEvent(t, events).Payload

	ctxB, cancelB := context.WithCancel(t.Context())
	resB := askAsync(ctxB, svc, "b")
	time.Sleep(100 * time.Millisecond)
	cancelB()
	require.ErrorIs(t, recvEvent(t, resB).err, context.Canceled)

	ansA := []Answer{{QuestionID: reqA.Questions[0].ID, FillInText: "answer-a"}}
	require.True(t, svc.Answer(ansA))
	require.Equal(t, ansA, recvEvent(t, resA).answers)
}

// TestStaleAnswerDoesNotResolveQueuedAsk verifies that a duplicate submit
// of A's answers cannot resolve the queued request that became current.
func TestStaleAnswerDoesNotResolveQueuedAsk(t *testing.T) {
	t.Parallel()
	svc := NewService()
	events := svc.Subscribe(t.Context())

	resA := askAsync(t.Context(), svc, "a")
	reqA := recvEvent(t, events).Payload
	resB := askAsync(t.Context(), svc, "b")
	time.Sleep(100 * time.Millisecond)

	ansA := []Answer{{QuestionID: reqA.Questions[0].ID, FillInText: "answer-a"}}
	require.True(t, svc.Answer(ansA))
	require.Equal(t, ansA, recvEvent(t, resA).answers)

	reqB := recvEvent(t, events).Payload
	require.Equal(t, "b", reqB.SessionID)
	require.False(t, svc.Answer(ansA), "stale answers must be rejected")
	pending, ok := svc.Pending()
	require.True(t, ok)
	require.Equal(t, reqB.ID, pending.ID)

	ansB := []Answer{{QuestionID: reqB.Questions[0].ID, FillInText: "answer-b"}}
	require.True(t, svc.Answer(ansB))
	require.Equal(t, ansB, recvEvent(t, resB).answers)
}
