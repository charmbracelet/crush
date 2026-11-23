package question

import (
	"slices"
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

	require.Eventually(t, func() bool { return len(svc.Pending()) == 1 },
		2*time.Second, 5*time.Millisecond)
	pending := svc.Pending()[0]

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

	require.Eventually(t, func() bool { return len(svc.Pending()) == 1 },
		2*time.Second, 5*time.Millisecond)
	pending := svc.Pending()[0]

	require.True(t, svc.Cancel(pending.ID))
	require.ErrorIs(t, <-done, ErrCancelled)

	ev := <-notifications
	assert.Equal(t, pending.ID, ev.Payload.BatchID)
	assert.Equal(t, "s-cancel", ev.Payload.SessionID)
}

// TestAsk_ConcurrentBatchesAreRoutedByID guards that answers and cancels
// resolve the batch they name, not whichever batch was asked last.
func TestAsk_ConcurrentBatchesAreRoutedByID(t *testing.T) {
	t.Parallel()
	svc := NewService()
	ask := func(id string) <-chan error {
		errc := make(chan error, 1)
		go func() {
			req := freeTextRequest("s-" + id)
			req.ID = id
			_, err := svc.Ask(t.Context(), req)
			errc <- err
		}()
		require.Eventually(t, func() bool {
			return slices.ContainsFunc(svc.Pending(), func(r Request) bool { return r.ID == id })
		}, 2*time.Second, 5*time.Millisecond)
		return errc
	}
	errA, errB := ask("A"), ask("B")

	require.True(t, svc.Cancel("A"))
	require.Equal(t, "B", svc.Pending()[0].ID)
	require.ErrorIs(t, <-errA, ErrCancelled)
	require.True(t, svc.Answer("B", []Answer{{QuestionID: svc.Pending()[0].Questions[0].ID}}))
	require.NoError(t, <-errB)
}
