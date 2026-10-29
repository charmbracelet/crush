package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/stretchr/testify/require"
)

// TestPostWorkspaceQuestionsCancel_RoutesByBatchID guards against a cancel
// for one question batch ending another session's pending batch.
func TestPostWorkspaceQuestionsCancel_RoutesByBatchID(t *testing.T) {
	t.Parallel()
	c := newTestController()
	a := app.NewForTest(t.Context())
	t.Cleanup(a.ShutdownForTest)
	ws := installSyntheticWorkspace(t, c)
	ws.App = a

	published := a.Questions.Subscribe(t.Context())
	results := map[string]chan error{}
	for _, id := range []string{"A", "B"} {
		res := make(chan error, 1)
		results[id] = res
		go func() {
			_, err := a.Questions.Ask(t.Context(), question.Request{
				ID:        id,
				SessionID: "s-" + id,
				Questions: []question.Question{{ID: "q-" + id, Type: question.TypeYesNo, Text: "Proceed?", Description: "Batch " + id}},
			})
			res <- err
		}()
		select {
		case <-published:
		case <-time.After(10 * time.Second):
			t.Fatalf("batch %s was never published", id)
		}
	}

	cancel := func(batchID string) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/v1/workspaces/"+ws.ID+"/questions/cancel", strings.NewReader(`{"batch_request_id":"`+batchID+`"}`))
		req.SetPathValue("id", ws.ID)
		rec := httptest.NewRecorder()
		c.handlePostWorkspaceQuestionsCancel(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	cancel("A")
	select {
	case err := <-results["A"]:
		require.ErrorIs(t, err, question.ErrCancelled)
	case err := <-results["B"]:
		t.Fatalf("the cancel for batch A ended batch B: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("batch A was never cancelled")
	}
	select {
	case err := <-results["B"]:
		t.Fatalf("batch B ended: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}
