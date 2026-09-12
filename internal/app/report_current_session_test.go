package app

import (
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestReportCurrentSession_NoLookupWithoutHerdr pins that the
// session-title lookup only happens when a herdr client is attached.
// The title exists solely to feed herdr's pane metadata, and
// ReportCurrentSession runs on every session load, new and select, so
// outside a herdr pane the read is pure overhead.
func TestReportCurrentSession_NoLookupWithoutHerdr(t *testing.T) {
	t.Parallel()

	mock := &mockSessionService{sessions: []session.Session{{ID: "s1", Title: "Title"}}}
	app := newTestApp(mock)
	require.Nil(t, app.herdrClient)

	app.ReportCurrentSession(t.Context(), "s1")

	require.Empty(t, mock.gets)
}
