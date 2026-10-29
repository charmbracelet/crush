package backend

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/stretchr/testify/require"
)

// TestAnswerQuestion_RoutesByBatchRequestID guards against an answer
// reaching the latest pending batch instead of the one it names, with
// questions pending in two sessions at once.
func TestAnswerQuestion_RoutesByBatchRequestID(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	published := a.Questions.Subscribe(t.Context())

	results := map[string]chan []question.Answer{}
	for _, id := range []string{"A", "B"} {
		res := make(chan []question.Answer, 1)
		results[id] = res
		go func() {
			answers, _ := a.Questions.Ask(t.Context(), question.Request{
				ID: id, SessionID: "s-" + id,
				Questions: []question.Question{{ID: "q-" + id, Type: question.TypeYesNo, Text: "Proceed?", Description: "d"}},
			})
			res <- answers
		}()
		select {
		case <-published:
		case <-time.After(10 * time.Second):
			t.Fatalf("batch %s was never published", id)
		}
	}

	yes := true
	resolved, err := b.AnswerQuestion(ws.ID, proto.QuestionAnswer{
		BatchRequestID: "A",
		Responses:      []proto.QuestionResponse{{QuestionID: "q-A", Yes: &yes}},
	})
	require.NoError(t, err)
	require.True(t, resolved)
	select {
	case got := <-results["A"]:
		require.Len(t, got, 1)
		require.Equal(t, "q-A", got[0].QuestionID)
	case <-results["B"]:
		t.Fatal("the answer to batch A reached batch B")
	case <-time.After(10 * time.Second):
		t.Fatal("batch A was never resolved")
	}
}
