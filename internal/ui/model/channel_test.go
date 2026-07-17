package model

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/textarea"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
)

// channelWorkspace is a workspace.Workspace stub that records the calls
// handleChannelMessage makes. Only the methods that path touches are
// implemented; the embedded interface panics on anything else, which keeps the
// stub honest about what the code under test depends on.
type channelWorkspace struct {
	workspace.Workspace
	ready       bool
	createErr   error
	newSession  session.Session
	createCalls int
	runCalls    []channelRun
	runErr      error
}

type channelRun struct {
	sessionID string
	prompt    string
}

func (w *channelWorkspace) AgentIsReady() bool { return w.ready }

func (w *channelWorkspace) CreateSession(context.Context, string) (session.Session, error) {
	w.createCalls++
	if w.createErr != nil {
		return session.Session{}, w.createErr
	}
	return w.newSession, nil
}

func (w *channelWorkspace) AgentRun(_ context.Context, sessionID, prompt string, _ ...message.Attachment) error {
	w.runCalls = append(w.runCalls, channelRun{sessionID: sessionID, prompt: prompt})
	return w.runErr
}

func (w *channelWorkspace) Config() *config.Config { return nil }

// newChannelUI builds a UI initialized enough that ensureSession's setState /
// layout path is safe to exercise.
func newChannelUI(ws *channelWorkspace) *UI {
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

func TestHandleChannelMessageExistingSession(t *testing.T) {
	t.Parallel()
	ws := &channelWorkspace{ready: true}
	m := newChannelUI(ws)
	m.session = &session.Session{ID: "sess-1"}

	cmd := m.handleChannelMessage(mcp.Event{ChannelMessage: "<channel source=\"s\">hi</channel>"})
	if cmd == nil {
		t.Fatal("expected a command for an active-session channel event")
	}
	// No session creation when one is already active.
	if ws.createCalls != 0 {
		t.Errorf("CreateSession calls = %d, want 0", ws.createCalls)
	}
	// Running the command drives the AgentRun injection.
	cmd()
	if len(ws.runCalls) != 1 {
		t.Fatalf("AgentRun calls = %d, want 1", len(ws.runCalls))
	}
	if ws.runCalls[0].sessionID != "sess-1" {
		t.Errorf("AgentRun sessionID = %q, want sess-1", ws.runCalls[0].sessionID)
	}
	if ws.runCalls[0].prompt != "<channel source=\"s\">hi</channel>" {
		t.Errorf("AgentRun prompt = %q", ws.runCalls[0].prompt)
	}
}

func TestHandleChannelMessageCreatesSessionWhenNoneActive(t *testing.T) {
	t.Parallel()
	ws := &channelWorkspace{ready: true, newSession: session.Session{ID: "new-sess"}}
	m := newChannelUI(ws)
	// No active session: a pushed event must not be dropped.

	cmd := m.handleChannelMessage(mcp.Event{ChannelMessage: "<channel source=\"s\">hi</channel>"})
	if cmd == nil {
		t.Fatal("expected a command when auto-creating a session")
	}
	if ws.createCalls != 1 {
		t.Errorf("CreateSession calls = %d, want 1", ws.createCalls)
	}
	if !m.hasSession() || m.session.ID != "new-sess" {
		t.Fatalf("expected active session new-sess, got %+v", m.session)
	}
}

func TestHandleChannelMessageDropsWhenNotReadyOrEmpty(t *testing.T) {
	t.Parallel()

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()
		ws := &channelWorkspace{ready: true}
		m := newChannelUI(ws)
		if cmd := m.handleChannelMessage(mcp.Event{ChannelMessage: ""}); cmd != nil {
			t.Error("empty channel message should be a no-op")
		}
		if ws.createCalls != 0 {
			t.Error("empty message should not create a session")
		}
	})

	t.Run("agent not ready", func(t *testing.T) {
		t.Parallel()
		ws := &channelWorkspace{ready: false}
		m := newChannelUI(ws)
		if cmd := m.handleChannelMessage(mcp.Event{ChannelMessage: "<channel source=\"s\">hi</channel>"}); cmd != nil {
			t.Error("event should be dropped when the agent is not ready")
		}
		if ws.createCalls != 0 {
			t.Error("not-ready event should not create a session")
		}
	})
}
