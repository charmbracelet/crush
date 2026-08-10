package herdr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestSummaryMessageLifecycleIntegration pins the contract between
// message.Service's publish behaviour and Translate that the
// compaction fix relies on: an unfinished summary message means
// compaction started, a finished one means it completed or errored,
// and a deleted one means it was cancelled. Drives a real
// SQLite-backed message service through both the success and the
// cancel paths of sessionAgent.Summarize and asserts the translated
// herdr events plus the resulting client state sequence.
func TestSummaryMessageLifecycleIntegration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(ctx, "test")
	require.NoError(t, err)

	// Zero debounce so every Update publishes synchronously.
	svc := message.NewService(q, message.WithDebounce(0))
	ch := svc.Subscribe(ctx)

	c := newTestClient()
	c.SetSessionID(sess.ID)

	// drain pops one published event and forwards its translation.
	drain := func() {
		t.Helper()
		select {
		case ev := <-ch:
			if hev := Translate(ev); hev != nil {
				c.HandleEvent(hev)
			}
		default:
			t.Fatal("expected a published message event")
		}
	}

	// Success path: create, stream content, finish (agent.go:1387,
	// 1450).
	msg, err := svc.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		IsSummaryMessage: true,
	})
	require.NoError(t, err)
	drain() // CreatedEvent -> SummarizeStarted.

	msg.AppendContent("summary text")
	require.NoError(t, svc.Update(ctx, msg))
	drain() // UpdatedEvent, unfinished -> SummarizeStarted (deduped).

	msg.AddFinish(message.FinishReasonEndTurn, "", "")
	require.NoError(t, svc.Update(ctx, msg))
	drain() // UpdatedEvent, finished -> SummarizeFinished.

	// Cancel path: create, then delete (agent.go:1437-1440).
	cancelMsg, err := svc.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		IsSummaryMessage: true,
	})
	require.NoError(t, err)
	drain() // CreatedEvent -> SummarizeStarted.

	require.NoError(t, svc.Delete(ctx, cancelMsg.ID))
	drain() // DeletedEvent -> SummarizeFinished.

	require.Equal(t,
		[]string{stateWorking, stateIdle, stateWorking, stateIdle},
		reportedStates(c),
	)
}

// TestTranslateDomainDeletedNonSummary guards the boundary of the
// delete mapping: deleting a regular assistant message (session
// cleanup) is not a compaction signal and keeps its pre-existing
// translation.
func TestTranslateDomainDeletedNonSummary(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[message.Message]{
		Type:    pubsub.DeletedEvent,
		Payload: message.Message{Role: message.Assistant, SessionID: "s1"},
	}
	require.Equal(t, AssistantMessage{SessionID: "s1"}, Translate(ev))
}
