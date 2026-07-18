package app

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newTestAppWithHistory wires a real session.Service and history.Service
// against the same sqlite connection, so that session IDs created via one
// are valid foreign keys for file records created via the other.
func newTestAppWithHistory(t *testing.T) *App {
	t.Helper()

	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)

	q := db.New(conn)
	return &App{
		Sessions: session.NewService(q, conn),
		History:  history.NewService(q, conn),
	}
}

func TestApp_ListSessionHistory_AggregatesDirectChildFiles(t *testing.T) {
	app := newTestAppWithHistory(t)

	parent, err := app.Sessions.Create(t.Context(), "parent title")
	require.NoError(t, err)

	child, err := app.Sessions.CreateTaskSession(t.Context(), "tool-call-1", parent.ID, "child title")
	require.NoError(t, err)

	parentFile, err := app.History.Create(t.Context(), parent.ID, "parent.go", "parent content")
	require.NoError(t, err)

	childFile, err := app.History.Create(t.Context(), child.ID, "child.go", "child content")
	require.NoError(t, err)

	files, err := app.ListSessionHistory(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, files, 2)

	paths := []string{files[0].Path, files[1].Path}
	require.ElementsMatch(t, []string{parentFile.Path, childFile.Path}, paths)
}

func TestApp_ListSessionHistory_NoChildren(t *testing.T) {
	app := newTestAppWithHistory(t)

	parent, err := app.Sessions.Create(t.Context(), "parent title")
	require.NoError(t, err)

	file1, err := app.History.Create(t.Context(), parent.ID, "one.go", "one content")
	require.NoError(t, err)

	file2, err := app.History.Create(t.Context(), parent.ID, "two.go", "two content")
	require.NoError(t, err)

	files, err := app.ListSessionHistory(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, files, 2)

	paths := []string{files[0].Path, files[1].Path}
	require.ElementsMatch(t, []string{file1.Path, file2.Path}, paths)
}

func TestApp_ListSessionHistory_DoesNotRecurseGrandchildren(t *testing.T) {
	app := newTestAppWithHistory(t)

	parent, err := app.Sessions.Create(t.Context(), "parent title")
	require.NoError(t, err)

	child, err := app.Sessions.CreateTaskSession(t.Context(), "tool-call-1", parent.ID, "child title")
	require.NoError(t, err)

	grandchild, err := app.Sessions.CreateTaskSession(t.Context(), "tool-call-2", child.ID, "grandchild title")
	require.NoError(t, err)

	_, err = app.History.Create(t.Context(), parent.ID, "parent.go", "parent content")
	require.NoError(t, err)

	_, err = app.History.Create(t.Context(), child.ID, "child.go", "child content")
	require.NoError(t, err)

	_, err = app.History.Create(t.Context(), grandchild.ID, "grandchild.go", "grandchild content")
	require.NoError(t, err)

	files, err := app.ListSessionHistory(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, files, 2)

	for _, f := range files {
		require.NotEqual(t, "grandchild.go", f.Path)
	}
}
