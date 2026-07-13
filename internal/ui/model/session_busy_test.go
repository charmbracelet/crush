package model

import (
	"testing"

	"charm.land/bubbles/v2/textarea"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
)

// busyWorkspace is a workspace.Workspace stub that reports readiness and
// per-session busy state. Only the methods isCurrentSessionBusy touches are
// implemented; the embedded interface panics on anything else.
type busyWorkspace struct {
	workspace.Workspace
	ready    bool
	busy     map[string]bool
	busyCall []string
}

func (w *busyWorkspace) AgentIsReady() bool { return w.ready }

func (w *busyWorkspace) AgentIsSessionBusy(sessionID string) bool {
	w.busyCall = append(w.busyCall, sessionID)
	return w.busy[sessionID]
}

func (w *busyWorkspace) Config() *config.Config { return nil }

func newBusyUI(ws *busyWorkspace) *UI {
	com := common.DefaultCommon(ws)
	return &UI{
		com:      com,
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
		state:    uiChat,
		focus:    uiFocusEditor,
		width:    140,
		height:   45,
	}
}

// TestIsCurrentSessionBusyIsSessionScoped is the core of #28/#29: the current
// session's busy state must reflect only its own activity, never another
// session's, so a paused To-Do is not shown as actively executing.
func TestIsCurrentSessionBusyIsSessionScoped(t *testing.T) {
	t.Parallel()

	t.Run("current session busy", func(t *testing.T) {
		t.Parallel()
		ws := &busyWorkspace{ready: true, busy: map[string]bool{"sess-1": true}}
		m := newBusyUI(ws)
		m.session = &session.Session{ID: "sess-1"}
		if !m.isCurrentSessionBusy() {
			t.Fatal("expected the viewed session's own activity to read as busy")
		}
	})

	t.Run("another session busy does not affect current", func(t *testing.T) {
		t.Parallel()
		ws := &busyWorkspace{ready: true, busy: map[string]bool{"other": true}}
		m := newBusyUI(ws)
		m.session = &session.Session{ID: "sess-1"}
		if m.isCurrentSessionBusy() {
			t.Fatal("another session's activity must not mark the current session busy")
		}
		// The check must be scoped to the current session ID.
		for _, id := range ws.busyCall {
			if id != "sess-1" {
				t.Fatalf("AgentIsSessionBusy queried %q, want only sess-1", id)
			}
		}
	})

	t.Run("agent not ready", func(t *testing.T) {
		t.Parallel()
		ws := &busyWorkspace{ready: false, busy: map[string]bool{"sess-1": true}}
		m := newBusyUI(ws)
		m.session = &session.Session{ID: "sess-1"}
		if m.isCurrentSessionBusy() {
			t.Fatal("a not-ready agent must not read as busy")
		}
	})

	t.Run("no active session", func(t *testing.T) {
		t.Parallel()
		ws := &busyWorkspace{ready: true, busy: map[string]bool{"sess-1": true}}
		m := newBusyUI(ws)
		if m.isCurrentSessionBusy() {
			t.Fatal("no active session must not read as busy")
		}
	})
}
