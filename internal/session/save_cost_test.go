package session

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func newCostTestService(t *testing.T) Service {
	t.Helper()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	return NewService(db.New(conn), conn)
}

func TestSaveKeepsCostAddedAfterSessionWasRead(t *testing.T) {
	t.Parallel()
	sessions := newCostTestService(t)

	created, err := sessions.Create(t.Context(), "before")
	require.NoError(t, err)

	stale, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)

	require.NoError(t, sessions.AddCost(t.Context(), created.ID, 0.40))

	stale.Title = "after"
	stale.Cost = 0
	saved, err := sessions.Save(t.Context(), stale)
	require.NoError(t, err)
	require.InDelta(t, 0.40, saved.Cost, 1e-9, "the returned session reports the current cost")

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "after", fetched.Title)
	require.InDelta(t, 0.40, fetched.Cost, 1e-9, "a stale Save must not overwrite an atomic AddCost")
}

func TestSaveIgnoresCostOnTheSessionValue(t *testing.T) {
	t.Parallel()
	sessions := newCostTestService(t)

	created, err := sessions.Create(t.Context(), "cost")
	require.NoError(t, err)
	require.NoError(t, sessions.AddCost(t.Context(), created.ID, 0.25))

	created.Cost = 9.99
	saved, err := sessions.Save(t.Context(), created)
	require.NoError(t, err)
	require.InDelta(t, 0.25, saved.Cost, 1e-9)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.InDelta(t, 0.25, fetched.Cost, 1e-9, "only AddCost changes the stored cost")
}

func TestSaveStillPersistsNonCostFields(t *testing.T) {
	t.Parallel()
	sessions := newCostTestService(t)

	created, err := sessions.Create(t.Context(), "fields")
	require.NoError(t, err)

	created.Title = "renamed"
	created.PromptTokens = 120
	created.CompletionTokens = 45
	created.SummaryMessageID = "summary-1"
	created.Channel = "signal"
	created.Todos = []Todo{{
		Content:    "Write tests",
		Status:     TodoStatusInProgress,
		ActiveForm: "Writing tests",
	}}

	_, err = sessions.Save(t.Context(), created)
	require.NoError(t, err)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "renamed", fetched.Title)
	require.Equal(t, int64(120), fetched.PromptTokens)
	require.Equal(t, int64(45), fetched.CompletionTokens)
	require.Equal(t, "summary-1", fetched.SummaryMessageID)
	require.Equal(t, "signal", fetched.Channel)
	require.Equal(t, created.Todos, fetched.Todos)
}
