package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func newPersistTestAgent(t *testing.T) (*sessionAgent, session.Service) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	sessions := session.NewService(db.New(conn), conn)
	a := &sessionAgent{sessions: sessions}
	return a, sessions
}

func testModelPair() (Model, Model) {
	large := Model{ModelCfg: config.SelectedModel{
		Provider:        "test-provider",
		Model:           "test-large",
		ReasoningEffort: "high",
		MaxTokens:       4096,
	}}
	small := Model{ModelCfg: config.SelectedModel{
		Provider: "test-provider",
		Model:    "test-small",
	}}
	return large, small
}

func TestPersistSessionModels_TopLevelSavesBoth(t *testing.T) {
	t.Parallel()

	a, sessions := newPersistTestAgent(t)
	ctx := context.Background()

	sess, err := sessions.Create(ctx, "Top level")
	require.NoError(t, err)

	large, small := testModelPair()
	require.NoError(t, a.persistSessionModels(ctx, sess, large, small))

	got, err := sessions.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byType := map[config.SelectedModelType]session.SessionModel{}
	for _, m := range got {
		byType[m.ModelType] = m
	}

	gotLarge, ok := byType[config.SelectedModelTypeLarge]
	require.True(t, ok)
	require.Equal(t, "test-large", gotLarge.Model)
	require.Equal(t, "high", gotLarge.SelectedModel.ReasoningEffort)
	require.Equal(t, int64(4096), gotLarge.SelectedModel.MaxTokens)

	gotSmall, ok := byType[config.SelectedModelTypeSmall]
	require.True(t, ok)
	require.Equal(t, "test-small", gotSmall.Model)
}

func TestPersistSessionModels_SubAgentSkipped(t *testing.T) {
	t.Parallel()

	a, sessions := newPersistTestAgent(t)
	a.isSubAgent = true
	ctx := context.Background()

	sess, err := sessions.Create(ctx, "Sub agent")
	require.NoError(t, err)

	large, small := testModelPair()
	require.NoError(t, a.persistSessionModels(ctx, sess, large, small))

	got, err := sessions.ListModels(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, got, "sub-agent runs must not write session_models")
}

func TestPersistSessionModels_ChildSessionSkipped(t *testing.T) {
	t.Parallel()

	a, sessions := newPersistTestAgent(t)
	ctx := context.Background()

	parent, err := sessions.Create(ctx, "Parent")
	require.NoError(t, err)
	child, err := sessions.CreateTaskSession(ctx, "tool-call-id", parent.ID, "Child")
	require.NoError(t, err)

	large, small := testModelPair()
	require.NoError(t, a.persistSessionModels(ctx, child, large, small))

	got, err := sessions.ListModels(ctx, child.ID)
	require.NoError(t, err)
	require.Empty(t, got, "child sessions inherit parent selections; no rows expected")
}

func TestPersistSessionModels_AgentToolSessionSkipped(t *testing.T) {
	t.Parallel()

	a, sessions := newPersistTestAgent(t)
	ctx := context.Background()

	// Build a synthetic agent-tool session ID. The session does not exist
	// in the DB, but persistSessionModels must short-circuit before
	// touching the store, so no error and no rows are expected.
	toolSessionID := sessions.CreateAgentToolSessionID("msg-1", "call-1")
	sess := session.Session{ID: toolSessionID}

	large, small := testModelPair()
	require.NoError(t, a.persistSessionModels(ctx, sess, large, small))

	got, err := sessions.ListModels(ctx, toolSessionID)
	require.NoError(t, err)
	require.Empty(t, got)
}
