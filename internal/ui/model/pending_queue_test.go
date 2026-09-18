package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/stretchr/testify/require"
)

// A prompt sent while the agent is working is queued behind the running turn.
// That turn already has a spinner and a running clock, so the placeholder
// must not be added: it would show a second spinner that sits there until the
// queued prompt finally ran, and restart the elapsed time mid-turn.
func TestPendingSpinnerSkippedWhileBusy(t *testing.T) {
	pinTTLs(t)

	for _, tc := range []struct {
		name                  string
		busy, wantPlaceholder bool
	}{
		{"idle shows the placeholder", false, true},
		{"busy leaves the running turn's spinner alone", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := &countingWorkspace{ready: true}
			m := newBusyUI(ws)
			warmCaches(m, tc.busy)
			m.session = &session.Session{ID: "s1"}

			m.sendMessage("hello")

			got := m.chat.MessageItem(chat.PendingAssistantID) != nil
			require.Equal(t, tc.wantPlaceholder, got)
		})
	}
}
