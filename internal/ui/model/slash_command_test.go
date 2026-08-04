package model

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/textarea"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
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

func (w *slashCommandWorkspace) PermissionSkipRequests() bool { return false }

func (w *slashCommandWorkspace) CreateSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
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

// TestSlashClearBlockedWhenAgentBusy verifies /clear warns when busy, keeps
// the session, and still resets the prompt history.
func TestSlashClearBlockedWhenAgentBusy(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true, busy: map[string]bool{"s1": true}})
	m.session = &session.Session{ID: "s1"}
	// The gate is a pure cache read of the session-scoped busy state
	// (workspace probes happen off-thread); seed it the way a refresh would.
	m.sessionBusyCache.setForSession(true, "s1")
	m.promptHistory.index = 3
	m.promptHistory.draft = "wip"

	_, ok := m.handleSlashCommand("/clear")
	if !ok {
		t.Fatal("handleSlashCommand(/clear) should still match when busy")
	}
	if m.session == nil {
		t.Fatal("session should not be cleared when agent is busy")
	}
	if m.promptHistory.index != -1 || m.promptHistory.draft != "" {
		t.Fatalf("prompt history not reset on busy /clear: index=%d draft=%q", m.promptHistory.index, m.promptHistory.draft)
	}
}

// TestSlashClearNotBlockedByOtherSessionBusy verifies the busy gate is
// session-scoped: agent activity in a different session must not block
// /clear for the session the user is viewing.
func TestSlashClearNotBlockedByOtherSessionBusy(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true, busy: map[string]bool{"other": true}})
	m.session = &session.Session{ID: "s1"}
	// A busy value cached for a DIFFERENT session must not gate this one.
	m.sessionBusyCache.setForSession(true, "other")

	_, ok := m.handleSlashCommand("/clear")
	if !ok {
		t.Fatal("handleSlashCommand(/clear) should match")
	}
	if m.session != nil {
		t.Fatal("busy state of another session must not block /clear")
	}
	if m.state != uiLanding {
		t.Fatal("expected state to be uiLanding after /clear")
	}
}

// TestSlashClearNoSessionResetsHistory verifies /clear with no active session
// is a no-op for session state but still resets the prompt history/draft —
// newSession early-returns without resetting, so /clear handles it directly.
func TestSlashClearNoSessionResetsHistory(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true})
	m.promptHistory.index = 3
	m.promptHistory.draft = "wip"

	cmd, ok := m.handleSlashCommand("/clear")
	if !ok {
		t.Fatal("handleSlashCommand(/clear) should match without a session")
	}
	if cmd != nil {
		t.Fatal("expected no command when there is no session to clear")
	}
	if m.promptHistory.index != -1 || m.promptHistory.draft != "" {
		t.Fatalf("prompt history not reset on no-session /clear: index=%d draft=%q", m.promptHistory.index, m.promptHistory.draft)
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
	cmd()
	if ws.summCall != "s1" {
		t.Fatalf("expected AgentSummarize called with s1, got %q", ws.summCall)
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

// TestSlashCompactNotBlockedByOtherSessionBusy verifies the busy gate is
// session-scoped: agent activity in a different session must not block
// /compact for the session the user is viewing.
func TestSlashCompactNotBlockedByOtherSessionBusy(t *testing.T) {
	t.Parallel()

	ws := &slashCommandWorkspace{ready: true, busy: map[string]bool{"other": true}}
	m := newSlashCommandUI(ws)
	m.session = &session.Session{ID: "s1"}
	// A busy value cached for a DIFFERENT session must not gate this one.
	m.sessionBusyCache.setForSession(true, "other")

	cmd, ok := m.handleSlashCommand("/compact")
	if !ok {
		t.Fatal("handleSlashCommand(/compact) should match")
	}
	if cmd == nil {
		t.Fatal("expected a summarize command despite another session being busy")
	}
	cmd()
	if ws.summCall != "s1" {
		t.Fatalf("expected AgentSummarize called with s1, got %q", ws.summCall)
	}
}

// TestNewSessionKeyNotBlockedByOtherSessionBusy verifies the ctrl+n
// new-session gate is session-scoped like /clear (both run newSession):
// agent activity in a different session must not block it.
func TestNewSessionKeyNotBlockedByOtherSessionBusy(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true, busy: map[string]bool{"other": true}})
	m.keyMap = DefaultKeyMap()
	m.dialog = dialog.NewOverlay()
	m.attachments = attachments.New(nil, attachments.Keymap{})
	m.session = &session.Session{ID: "s1"}
	// A busy value cached for a DIFFERENT session must not gate this one.
	m.sessionBusyCache.setForSession(true, "other")

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})

	if m.session != nil {
		t.Fatal("busy state of another session must not block ctrl+n new session")
	}
	if m.state != uiLanding {
		t.Fatal("expected state to be uiLanding after new session")
	}
}

// TestBangModeWinsOverSlashCommands verifies that in bang (!) shell mode a
// command that is literally "/clear" is executed in the shell instead of
// being hijacked by the slash-command handler.
func TestBangModeWinsOverSlashCommands(t *testing.T) {
	t.Parallel()

	m := newSlashCommandUI(&slashCommandWorkspace{ready: true})
	m.keyMap = DefaultKeyMap()
	m.dialog = dialog.NewOverlay()
	m.attachments = attachments.New(nil, attachments.Keymap{})
	m.bangMode = true
	m.textarea.SetValue("/clear")

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.bangMode {
		t.Fatal("bang mode should be consumed by Enter")
	}
	if m.pendingBangCommand != "/clear" {
		t.Fatalf("bang-mode input should run as a shell command, got pending %q", m.pendingBangCommand)
	}
}

// TestSlashCompactBlockedWhenAgentBusy verifies /compact warns when busy and
// does not call AgentSummarize even if the returned cmd is executed.
func TestSlashCompactBlockedWhenAgentBusy(t *testing.T) {
	t.Parallel()

	ws := &slashCommandWorkspace{ready: true, busy: map[string]bool{"s1": true}}
	m := newSlashCommandUI(ws)
	m.session = &session.Session{ID: "s1"}
	// The gate is a pure cache read of the session-scoped busy state
	// (workspace probes happen off-thread); seed it the way a refresh would.
	m.sessionBusyCache.setForSession(true, "s1")

	cmd, ok := m.handleSlashCommand("/compact")
	if !ok {
		t.Fatal("handleSlashCommand(/compact) should still match when busy")
	}
	if cmd != nil {
		cmd()
	}
	if ws.summCall != "" {
		t.Fatal("AgentSummarize should not be called when agent is busy")
	}
}
