package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestSummaryExportPath(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		filepath.Join(".crush", "summaries", "sess-123.md"),
		summaryExportPath(".crush", "sess-123"),
	)
}

func TestSaveSummaryExport(t *testing.T) {
	t.Parallel()

	summaryMsg := message.Message{
		ID:               "msg-1",
		SessionID:        "sess-123",
		Role:             message.Assistant,
		IsSummaryMessage: true,
		Parts:            []message.ContentPart{message.TextContent{Text: "## Summary\n\nWe refactored the parser.\n"}},
	}
	sess := session.Session{ID: "sess-123", SummaryMessageID: "msg-1"}
	msgs := []message.Message{
		{ID: "msg-0", SessionID: "sess-123", Role: message.User},
		summaryMsg,
	}

	t.Run("writes summary content to file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		path, err := saveSummaryExport(dir, sess, msgs)
		require.NoError(t, err)
		require.Equal(t, summaryExportPath(dir, "sess-123"), path)

		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "## Summary\n\nWe refactored the parser.", string(b))
	})

	t.Run("overwrites previous summary", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		updated := sess
		updatedMsgs := []message.Message{
			{ID: "msg-2", SessionID: "sess-123", IsSummaryMessage: true, Parts: []message.ContentPart{message.TextContent{Text: "newer summary"}}},
		}
		updated.SummaryMessageID = "msg-2"

		path, err := saveSummaryExport(dir, sess, msgs)
		require.NoError(t, err)

		path2, err := saveSummaryExport(dir, updated, updatedMsgs)
		require.NoError(t, err)
		require.Equal(t, path, path2)

		b, err := os.ReadFile(path2)
		require.NoError(t, err)
		require.Equal(t, "newer summary", string(b))
	})

	t.Run("errors when session has no summary", func(t *testing.T) {
		t.Parallel()

		noSummary := session.Session{ID: "sess-123"}
		_, err := saveSummaryExport(t.TempDir(), noSummary, msgs)
		require.Error(t, err)
	})

	t.Run("errors when summary message is missing", func(t *testing.T) {
		t.Parallel()

		_, err := saveSummaryExport(t.TempDir(), sess, nil)
		require.Error(t, err)
	})

	t.Run("errors when summary has no text content", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		empty := []message.Message{
			{ID: "msg-1", SessionID: "sess-123", IsSummaryMessage: true},
		}
		_, err := saveSummaryExport(dir, sess, empty)
		require.Error(t, err)
	})
}
