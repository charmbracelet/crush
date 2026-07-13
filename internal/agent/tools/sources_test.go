package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestAddSourceBatchAndResolve(t *testing.T) {
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	sessions := session.NewService(db.New(conn), conn)
	created, err := sessions.Create(t.Context(), "sources")
	require.NoError(t, err)

	imagePath := filepath.Join(t.TempDir(), "reference.png")
	require.NoError(t, os.WriteFile(imagePath, []byte("image metadata only"), 0o644))
	ctx := context.WithValue(t.Context(), SessionIDContextKey, created.ID)
	add := NewAddSourceTool(sessions, filepath.Dir(imagePath))
	response, err := add.Run(ctx, fantasy.ToolCall{
		Name: AddSourceToolName,
		Input: `{"items":[` +
			`{"value":"reference.png"},` +
			`{"value":"https://example.com/docs","label":"Docs"},` +
			`{"value":"Pinned note","kind":"text"}` +
			`]}`,
	})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, "Attached 3 source(s)")

	stored, err := sessions.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, stored.Sources, 3)
	require.Equal(t, session.SourceKindFile, stored.Sources[0].Kind)
	require.Equal(t, imagePath, stored.Sources[0].Location)
	require.Equal(t, session.SourceKindURL, stored.Sources[1].Kind)
	require.Equal(t, session.SourceKindText, stored.Sources[2].Kind)

	sources := NewSourcesTool(sessions)
	list, err := sources.Run(ctx, fantasy.ToolCall{Name: SourcesToolName, Input: `{}`})
	require.NoError(t, err)
	require.Contains(t, list.Content, "reference.png")
	require.Contains(t, list.Content, "Docs")

	resolved, err := sources.Run(ctx, fantasy.ToolCall{
		Name:  SourcesToolName,
		Input: `{"action":"resolve","id":"` + stored.Sources[2].ID + `"}`,
	})
	require.NoError(t, err)
	require.Contains(t, resolved.Content, "Pinned note")

	remove := NewRemoveSourceTool(sessions)
	removed, err := remove.Run(ctx, fantasy.ToolCall{
		Name:  RemoveSourceToolName,
		Input: `{"items":["example.com","reference"]}`,
	})
	require.NoError(t, err)
	require.Contains(t, removed.Content, "Detached 2 source(s)")
	stored, err = sessions.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, stored.Sources, 1)
	require.Equal(t, "Text source", stored.Sources[0].Label)
}
