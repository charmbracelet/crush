package herdr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
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
	c.SetSession(sess.ID, sess.Title)

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

// TestUserMessageRunStartedIntegration pins the prompt-submission
// contract between message.Service and Translate that the early
// working report relies on: creating a user message means a turn
// started, while a bang-mode shell record (also a user message) and
// an update to an existing prompt mean no run. Drives a real
// SQLite-backed message service and asserts the resulting client
// state sequence.
func TestUserMessageRunStartedIntegration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(ctx, "test")
	require.NoError(t, err)

	svc := message.NewService(q, message.WithDebounce(0))
	ch := svc.Subscribe(ctx)

	c := newTestClient()
	c.SetSession(sess.ID, sess.Title)

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

	// Prompt submission flips the pane to working before any
	// assistant output exists (agent.go:713).
	prompt, err := svc.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hi"}},
	})
	require.NoError(t, err)
	drain() // CreatedEvent -> RunStarted.

	// A bang-mode shell command persists a user message but starts
	// no run (shell.PersistOutput).
	_, err = svc.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.ShellCommand{Command: "ls", Output: "file.go", ExitCode: 0},
		},
	})
	require.NoError(t, err)
	drain() // CreatedEvent -> nil.

	// Update publishes an UpdatedEvent for whatever message it is
	// handed. Revising an existing prompt is not a submission, so it
	// must not re-arm working once the turn has ended.
	c.HandleEvent(RunComplete{SessionID: sess.ID})
	require.NoError(t, svc.Update(ctx, prompt))
	drain() // UpdatedEvent -> nil.

	require.Equal(t, []string{stateWorking, stateIdle}, reportedStates(c))
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

// TestSessionTitleChangeIntegration pins the contract between
// session.Service's publish behaviour and Translate that the pane
// title refresh relies on: a rename (or auto-title, which shares the
// same UpdatedEvent path) publishes the session with its new title,
// and only the current session's event updates the pane metadata.
// Drives a real SQLite-backed session service.
func TestSessionTitleChangeIntegration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	svc := session.NewService(q, conn)
	sess, err := svc.Create(ctx, "Untitled Session")
	require.NoError(t, err)
	other, err := svc.Create(ctx, "Other Session")
	require.NoError(t, err)

	ch := svc.Subscribe(ctx)

	c := newTestClient()
	c.SetSession(sess.ID, "Untitled Session")

	// drain pops one published event and forwards its translation.
	drain := func() {
		t.Helper()
		select {
		case ev := <-ch:
			if hev := Translate(ev); hev != nil {
				c.HandleEvent(hev)
			}
		default:
			t.Fatal("expected a published session event")
		}
	}

	// The two Creates published events for sessions that are not the
	// current one (the client's SetSession predates them but the
	// subscription started after, so nothing is drained for them).

	// A title change on a background session must not touch the
	// pane.
	require.NoError(t, svc.Rename(ctx, other.ID, "Background Title"))
	drain() // UpdatedEvent -> SessionUpdated, gated away.
	meta := reportedMetadata(c)
	require.Len(t, meta, 1)
	require.Equal(t, "Untitled Session", meta[0].Title)

	// A rename of the current session refreshes the pane title.
	require.NoError(t, svc.Rename(ctx, sess.ID, "Generated Title"))
	drain() // UpdatedEvent -> SessionUpdated, applied.
	meta = reportedMetadata(c)
	require.Len(t, meta, 2)
	require.Equal(t, "Generated Title", meta[1].Title)
	if assert.NotNil(t, meta[1].Tokens["session"]) {
		assert.Equal(t, sess.ID, *meta[1].Tokens["session"])
	}
}

// TestPermissionRequestPublishOrderIntegration pins the contract
// between permission.Service's publish behaviour and Translate that
// the blocked report relies on. The service announces a request on
// the notification broker before publishing the request itself, and
// BridgeLocal consumes the two brokers on separate goroutines, so
// the announcement may reach the client after the request does. It
// must not unblock the pane; only the granted or denied notification
// may. Drives a real permission service through a full
// request/grant cycle and replays both events in the adverse order.
func TestPermissionRequestPublishOrderIntegration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dir := t.TempDir()
	svc := permission.NewPermissionService(dir, false, nil)
	requests := svc.Subscribe(ctx)
	notifications := svc.SubscribeNotifications(ctx)

	c := newTestClient()
	c.SetSession("s1", "Test")
	c.HandleEvent(RunStarted{SessionID: "s1"})

	granted := make(chan bool, 1)
	go func() {
		ok, err := svc.Request(ctx, permission.CreatePermissionRequest{
			SessionID:   "s1",
			ToolCallID:  "tc-1",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Execute command: ls",
			Path:        dir,
		})
		assert.NoError(t, err)
		granted <- ok
	}()

	// deliver forwards one event exactly as BridgeLocal's forward
	// goroutines do.
	deliver := func(ev any) {
		t.Helper()
		if hev := Translate(ev); hev != nil {
			c.HandleEvent(hev)
		}
	}

	announce := <-notifications
	require.Equal(t, "tc-1", announce.Payload.ToolCallID)
	require.False(t, announce.Payload.Granted)
	require.False(t, announce.Payload.Denied)
	req := <-requests

	// Adverse order: the request blocks the pane, then the
	// announcement of that same request arrives late.
	deliver(req)
	deliver(announce)
	require.Equal(t, []string{stateWorking, stateBlocked}, reportedStates(c))
	// The real service's payload reaches herdr as the blocked
	// reason, so the pane shows what crush is waiting on.
	require.Equal(t,
		"Permission: bash - Execute command: ls",
		lastRequest(c).Params.(reportParams).Message,
	)

	require.True(t, svc.Grant(req.Payload))
	deliver(<-notifications)
	require.True(t, <-granted)
	require.Equal(t,
		[]string{stateWorking, stateBlocked, stateWorking},
		reportedStates(c),
	)
}
