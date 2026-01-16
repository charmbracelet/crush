package sessions

import (
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
)

func TestNewSessionDialogCmp(t *testing.T) {
	t.Parallel()

	t.Run("creates dialog with sessions", func(t *testing.T) {
		t.Parallel()
		sessions := []session.Session{
			{ID: "session-1", Title: "First Session"},
			{ID: "session-2", Title: "Second Session"},
		}

		dialog := NewSessionDialogCmp(sessions, "", nil)
		assert.NotNil(t, dialog)
		assert.Equal(t, SessionsDialogID, dialog.ID())
	})

	t.Run("creates dialog with empty sessions", func(t *testing.T) {
		t.Parallel()
		dialog := NewSessionDialogCmp([]session.Session{}, "", nil)
		assert.NotNil(t, dialog)
	})

	t.Run("marks busy sessions with indicator", func(t *testing.T) {
		t.Parallel()
		sessions := []session.Session{
			{ID: "session-1", Title: "Active Session"},
			{ID: "session-2", Title: "Idle Session"},
			{ID: "session-3", Title: "Another Active"},
		}
		busyIDs := []string{"session-1", "session-3"}

		dialog := NewSessionDialogCmp(sessions, "", busyIDs)
		assert.NotNil(t, dialog)

		// Verify dialog was created - the busy indicator logic is internal
		// but we can verify the dialog handles the busy IDs without panicking
		cmp, ok := dialog.(*sessionDialogCmp)
		assert.True(t, ok)
		assert.Equal(t, busyIDs, cmp.busySessionIDs)
	})

	t.Run("handles nil busy session IDs", func(t *testing.T) {
		t.Parallel()
		sessions := []session.Session{
			{ID: "session-1", Title: "Test Session"},
		}

		// Should not panic with nil busySessionIDs
		dialog := NewSessionDialogCmp(sessions, "", nil)
		assert.NotNil(t, dialog)
	})

	t.Run("tracks selected session ID", func(t *testing.T) {
		t.Parallel()
		sessions := []session.Session{
			{ID: "session-1", Title: "First"},
			{ID: "session-2", Title: "Second"},
		}

		dialog := NewSessionDialogCmp(sessions, "session-2", nil)
		cmp, ok := dialog.(*sessionDialogCmp)
		assert.True(t, ok)
		assert.Equal(t, "session-2", cmp.selectedSessionID)
	})
}

func TestBusyIndicator(t *testing.T) {
	t.Parallel()

	// Verify the busy indicator constant is set correctly
	assert.Equal(t, "●", busyIndicator)
}
