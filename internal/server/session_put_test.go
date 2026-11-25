package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// savingSessions records the session the handler saves.
type savingSessions struct {
	session.Service
	saved session.Session
}

func (s *savingSessions) Save(_ context.Context, sess session.Session) (session.Session, error) {
	s.saved = sess
	return sess, nil
}

// TestSessionPutKeepsEveryField verifies that saving a session through the
// API, which is how clients rename one, stores the usage, summary and todos
// the client sent rather than zeroing them.
func TestSessionPutKeepsEveryField(t *testing.T) {
	t.Parallel()

	b := backend.New(context.Background(), nil, nil)
	sessions := &savingSessions{}
	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  &app.App{Sessions: sessions},
	}
	backend.InsertWorkspaceForTest(b, ws)
	c := &controllerV1{backend: b, server: &Server{backend: b}}

	body, err := json.Marshal(proto.Session{
		ID:               "s1",
		ParentSessionID:  "p1",
		Title:            "Renamed",
		MessageCount:     3,
		PromptTokens:     1234,
		CompletionTokens: 567,
		EstimatedUsage:   true,
		SummaryMessageID: "m-summary",
		Cost:             0.5,
		Todos:            []proto.Todo{{Content: "c", Status: "pending", ActiveForm: "doing c"}},
		Channel:          "ch",
		CreatedAt:        1,
		UpdatedAt:        2,
	})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/v1/workspaces/"+ws.ID+"/sessions/s1", bytes.NewReader(body))
	req.SetPathValue("id", ws.ID)
	req.SetPathValue("sid", "s1")
	rec := httptest.NewRecorder()
	c.handlePutWorkspaceSession(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, session.Session{
		ID:               "s1",
		ParentSessionID:  "p1",
		Title:            "Renamed",
		MessageCount:     3,
		PromptTokens:     1234,
		CompletionTokens: 567,
		EstimatedUsage:   true,
		SummaryMessageID: "m-summary",
		Cost:             0.5,
		Todos:            []session.Todo{{Content: "c", Status: session.TodoStatusPending, ActiveForm: "doing c"}},
		Channel:          "ch",
		CreatedAt:        1,
		UpdatedAt:        2,
	}, sessions.saved)

	var got proto.Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, int64(1234), got.PromptTokens)
	require.Equal(t, "m-summary", got.SummaryMessageID)
	require.True(t, got.EstimatedUsage)
}
