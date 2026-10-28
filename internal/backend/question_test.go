package backend

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// askResult is the outcome of a [question.Service.Ask] call.
type askResult struct {
	answers []question.Answer
	err     error
}

// askPending asks batch id (one yes/no question with ID "q-<id>") in
// session "s-<id>" and returns once the batch is pending. published
// must be subscribed to svc before the call. The Ask's outcome arrives
// on the returned channel.
func askPending(t *testing.T, svc question.Service, published <-chan pubsub.Event[question.Request], id string) <-chan askResult {
	t.Helper()
	res := make(chan askResult, 1)
	go func() {
		answers, err := svc.Ask(t.Context(), question.Request{
			ID:        id,
			SessionID: "s-" + id,
			Questions: []question.Question{{
				ID:          "q-" + id,
				Type:        question.TypeYesNo,
				Text:        "Proceed?",
				Description: "Batch " + id,
			}},
		})
		res <- askResult{answers: answers, err: err}
	}()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev := <-published:
			if ev.Payload.ID == id {
				return res
			}
		case <-timeout:
			t.Fatalf("batch %s was never published", id)
		}
	}
}

// TestAnswerQuestion_RoutesByBatchRequestID verifies that, with
// questions pending in two sessions at once, an answer reaches the
// batch it names and leaves the other one pending.
func TestAnswerQuestion_RoutesByBatchRequestID(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	published := a.Questions.Subscribe(t.Context())

	resA := askPending(t, a.Questions, published, "A")
	resB := askPending(t, a.Questions, published, "B")

	yes := true
	resolved, err := b.AnswerQuestion(ws.ID, proto.QuestionAnswer{
		BatchRequestID: "A",
		Responses:      []proto.QuestionResponse{{QuestionID: "q-A", Yes: &yes}},
	})
	require.NoError(t, err)
	require.True(t, resolved)

	select {
	case got := <-resA:
		require.NoError(t, got.err)
		require.Len(t, got.answers, 1)
		require.Equal(t, "q-A", got.answers[0].QuestionID)
	case got := <-resB:
		t.Fatalf("the answer to batch A reached batch B (session s-B): %+v", got)
	case <-time.After(10 * time.Second):
		t.Fatal("batch A was never resolved")
	}

	// Batch B is still pending and takes its own answer.
	no := false
	resolved, err = b.AnswerQuestion(ws.ID, proto.QuestionAnswer{
		BatchRequestID: "B",
		Responses:      []proto.QuestionResponse{{QuestionID: "q-B", Yes: &no}},
	})
	require.NoError(t, err)
	require.True(t, resolved, "batch B must still be pending")

	select {
	case got := <-resB:
		require.NoError(t, got.err)
		require.Len(t, got.answers, 1)
		require.Equal(t, "q-B", got.answers[0].QuestionID)
		require.False(t, *got.answers[0].Yes)
	case <-time.After(10 * time.Second):
		t.Fatal("batch B was never resolved")
	}
}

// TestSetCurrentSession_RepublishesEachSessionsQuestion verifies that,
// with questions pending in two sessions at once, switching to either
// session re-delivers that session's batch.
func TestSetCurrentSession_RepublishesEachSessionsQuestion(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	ctx := t.Context()

	events, err := b.SubscribeEvents(ctx, ws.ID)
	require.NoError(t, err)
	published := a.Questions.Subscribe(ctx)

	askPending(t, a.Questions, published, "A")
	askPending(t, a.Questions, published, "B")

	// Absorb the original publishes, then attach a client.
	drainQuiet(events, 300*time.Millisecond)

	cid := uuid.New().String()
	require.NoError(t, b.AttachClient(ws.ID, cid))

	for _, id := range []string{"A", "B"} {
		require.NoError(t, b.SetCurrentSession(ctx, ws.ID, cid, "s-"+id))
		timeout := time.After(5 * time.Second)
		for republished := false; !republished; {
			select {
			case ev := <-events:
				// Skip anything else, such as an original publish the
				// fan-in delivered late.
				reqEv, ok := ev.Payload.(pubsub.Event[question.Request])
				republished = ok && reqEv.Payload.ID == id
			case <-timeout:
				t.Fatalf("batch %s was not re-published for session s-%s", id, id)
			}
		}
	}
}
