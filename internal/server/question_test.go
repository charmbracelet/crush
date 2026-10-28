package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/stretchr/testify/require"
)

// newQuestionTestWorkspace installs a workspace backed by a minimal
// app with a real question service.
func newQuestionTestWorkspace(t *testing.T, c *controllerV1) (*backend.Workspace, *app.App) {
	t.Helper()
	a := app.NewForTest(t.Context())
	t.Cleanup(a.ShutdownForTest)
	ws := installSyntheticWorkspace(t, c)
	ws.App = a
	return ws, a
}

// askQuestion asks batch id (one yes/no question with ID "q-<id>") in
// session "s-<id>" and returns once the batch is pending. The Ask's
// error arrives on the returned channel.
func askQuestion(t *testing.T, a *app.App, id string) <-chan error {
	t.Helper()
	published := a.Questions.Subscribe(t.Context())
	res := make(chan error, 1)
	go func() {
		_, err := a.Questions.Ask(t.Context(), question.Request{
			ID:        id,
			SessionID: "s-" + id,
			Questions: []question.Question{{
				ID:          "q-" + id,
				Type:        question.TypeYesNo,
				Text:        "Proceed?",
				Description: "Batch " + id,
			}},
		})
		res <- err
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

// postQuestionCancel drives the cancel handler. A nil body sends none,
// as clients that predate batch IDs do.
func postQuestionCancel(t *testing.T, c *controllerV1, wsID string, body any) bool {
	t.Helper()
	var r io.Reader = http.NoBody
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(data)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/workspaces/"+wsID+"/questions/cancel", r)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceQuestionsCancel(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp proto.QuestionAnswerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Resolved
}

// TestPostWorkspaceQuestionsCancel_RoutesByBatchID verifies that a
// cancel naming a batch cancels that batch only, leaving the question
// another session has pending untouched.
func TestPostWorkspaceQuestionsCancel_RoutesByBatchID(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws, a := newQuestionTestWorkspace(t, c)

	resA := askQuestion(t, a, "A")
	resB := askQuestion(t, a, "B")

	require.True(t, postQuestionCancel(t, c, ws.ID, map[string]string{"batch_request_id": "A"}))

	select {
	case err := <-resA:
		require.ErrorIs(t, err, question.ErrCancelled)
	case err := <-resB:
		t.Fatalf("the cancel for batch A ended batch B (session s-B): %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("batch A was never cancelled")
	}

	require.True(t, postQuestionCancel(t, c, ws.ID, map[string]string{"batch_request_id": "B"}),
		"batch B must still be pending")
	select {
	case err := <-resB:
		require.ErrorIs(t, err, question.ErrCancelled)
	case <-time.After(10 * time.Second):
		t.Fatal("batch B was never cancelled")
	}
}

// TestPostWorkspaceQuestionsCancel_WithoutBody verifies that a cancel
// without a body, as sent by clients that predate batch IDs, still
// cancels the only pending batch.
func TestPostWorkspaceQuestionsCancel_WithoutBody(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws, a := newQuestionTestWorkspace(t, c)

	res := askQuestion(t, a, "A")

	require.True(t, postQuestionCancel(t, c, ws.ID, nil))
	select {
	case err := <-res:
		require.ErrorIs(t, err, question.ErrCancelled)
	case <-time.After(10 * time.Second):
		t.Fatal("the pending batch was never cancelled")
	}
	require.False(t, postQuestionCancel(t, c, ws.ID, nil), "nothing is left to cancel")
}
