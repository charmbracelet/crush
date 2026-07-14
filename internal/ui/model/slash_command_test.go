package model

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/textarea"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
)

// slashCommandWorkspace is a minimal workspace stub for slash command tests.
type slashCommandWorkspace struct {
	workspace.Workspace
	ready    bool
	busy     map[string]bool
	summErr  error
	summCall string
}

func (w *slashCommandWorkspace) AgentIsReady() bool { return w.ready }

func (w *slashCommandWorkspace) AgentIsBusy() bool {
	for _, b := range w.busy {
		if b {
			return true
		}
	}
	return false
}

func (w *slashCommandWorkspace) AgentIsSessionBusy(sessionID string) bool {
	return w.busy[sessionID]
}

func (w *slashCommandWorkspace) Config() *config.Config { return nil }

func (w *slashCommandWorkspace) AgentSummarize(_ context.Context, sessionID string) error {
	w.summCall = sessionID
	return w.summErr
}

func newSlashCommandUI(ws *slashCommandWorkspace) *UI {
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

// TestSlashCommandNotMatched verifies that non-slash input returns false so
// normal message processing continues.
func TestSlashCommandNotMatched(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true})
	m.session = &session.Session{ID: "s1"}

	cases := []string{"hello", "/unknown", "  /clear extra", "/compact now", ""}
	for _, tc := range cases {
		_, ok := m.handleSlashCommand(tc)
		if ok {
			t.Fatalf("handleSlashCommand(%q) should not match", tc)
		}
	}
}

// TestSlashClearStartsNewSession verifies /clear resets to landing state.
func TestSlashClearStartsNewSession(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true})
	m.session = &session.Session{ID: "s1"}

	_, ok := m.handleSlashCommand("/clear")
	if !ok {
		t.Fatal("handleSlashCommand(/clear) should match")
	}
	if m.session != nil {
		t.Fatal("expected session to be nil after /clear")
	}
	if m.state != uiLanding {
		t.Fatal("expected state to be uiLanding after /clear")
	}
}

// TestSlashClearBlockedWhenAgentBusy verifies /clear warns when busy.
func TestSlashClearBlockedWhenAgentBusy(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true, busy: map[string]bool{"s1": true}})
	m.session = &session.Session{ID: "s1"}

	_, ok := m.handleSlashCommand("/clear")
	if !ok {
		t.Fatal("handleSlashCommand(/clear) should still match when busy")
	}
	if m.session == nil {
		t.Fatal("session should not be cleared when agent is busy")
	}
}

// TestSlashCompactTriggersSummarize verifies /compact calls AgentSummarize.
func TestSlashCompactTriggersSummarize(t *testing.T) {
	t.Parallel()

	ws := &slashCommandWorkspace{ready: true}
	m := newSlashCommandUI(ws)
	m.session = &session.Session{ID: "s1"}

	cmd, ok := m.handleSlashCommand("/compact")
	if !ok {
		t.Fatal("handleSlashCommand(/compact) should match")
	}
	if cmd == nil {
		t.Fatal("expected a command to be returned")
	}
}

// TestSlashCompactNoSession verifies /compact is a no-op without a session.
func TestSlashCompactNoSession(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true})

	_, ok := m.handleSlashCommand("/compact")
	if !ok {
		t.Fatal("handleSlashCommand(/compact) should still match without session")
	}
}

// TestSlashCompactBlockedWhenAgentBusy verifies /compact warns when busy.
func TestSlashCompactBlockedWhenAgentBusy(t *testing.T) {
	t.Parallel()

	ws := &slashCommandWorkspace{ready: true, busy: map[string]bool{"s1": true}}
	m := newSlashCommandUI(ws)
	m.session = &session.Session{ID: "s1"}

	_, ok := m.handleSlashCommand("/compact")
	if !ok {
		t.Fatal("handleSlashCommand(/compact) should still match when busy")
	}
	if ws.summCall != "" {
		t.Fatal("AgentSummarize should not be called when agent is busy")
	}
}
