package model

// Prompts (permission requests and questions) of a session that is not on
// screen open no dialog and take no keys: they are held by top-level session,
// the session shows waiting, a status line names it, and opening the session
// shows them. Leaving a session parks its open prompt the same way. In
// client/server mode the server already filters other sessions' prompts out
// and re-sends the pending one on a switch; this covers in-process mode, the
// switch race and the landing screen.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// promptRootMsg delivers the off-thread root lookup of a prompt raised by a
// sub-agent session. An empty root means the lookup failed.
type promptRootMsg struct {
	root string
	perm *permission.PermissionRequest
	q    *question.Request
}

// routePrompt opens a prompt of the session on screen and holds any other.
// Exactly one of perm and q is set.
func (m *UI) routePrompt(sessionID string, perm *permission.PermissionRequest, q *question.Request) tea.Cmd {
	// An unscoped prompt cannot be routed: show it, as before.
	if sessionID == "" || sessionID == m.currentSessionID() {
		m.openPrompt(perm, q)
		return nil
	}
	if m.isChildSession(sessionID) {
		return m.resolvePromptRoot(sessionID, perm, q)
	}
	return m.placePrompt(sessionID, perm, q)
}

// resolvePromptRoot looks up a sub-agent session's top-level session off the
// Update goroutine. The closure captures only locals.
func (m *UI) resolvePromptRoot(sessionID string, perm *permission.PermissionRequest, q *question.Request) tea.Cmd {
	ws := m.com.Workspace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		root, err := workspace.RootSessionID(ctx, ws, sessionID)
		if err != nil {
			slog.Debug("Failed to resolve prompt root session", "session_id", sessionID, "error", err)
			root = ""
		}
		return promptRootMsg{root: root, perm: perm, q: q}
	}
}

func (m *UI) applyPromptRoot(msg promptRootMsg) tea.Cmd {
	if msg.root == "" {
		// Unknown owner: shown, as before, rather than stranded. The server
		// fails open the same way.
		m.openPrompt(msg.perm, msg.q)
		return nil
	}
	return m.placePrompt(msg.root, msg.perm, msg.q)
}

// placePrompt opens the prompt when its top-level session is on screen, else
// holds it and marks the session waiting.
func (m *UI) placePrompt(root string, perm *permission.PermissionRequest, q *question.Request) tea.Cmd {
	if root == m.currentSessionID() {
		m.openPrompt(perm, q)
		return nil
	}
	m.holdPrompt(root, perm, q)
	if m.activity.Get(root) == activityWaiting {
		return nil
	}
	m.activity.Waiting(root)
	return m.waitingInfo(root)
}

// openPrompt shows a prompt of the session on screen, unless it already is
// (the server re-sends the pending prompt on a switch).
func (m *UI) openPrompt(perm *permission.PermissionRequest, q *question.Request) {
	if perm != nil {
		if open, ok := m.dialog.Dialog(dialog.PermissionsID).(*dialog.Permissions); !ok || perm.ID == "" || open.Permission().ID != perm.ID {
			m.openPermissionsDialog(*perm)
		}
	}
	if q != nil {
		if open, ok := m.activeInline.(*dialog.QuestionForm); !ok || q.ID == "" || open.BatchID != q.ID {
			m.openBatchFormDialog(*q)
			m.chat.ScrollToBottom()
		}
	}
}

func (m *UI) holdPrompt(root string, perm *permission.PermissionRequest, q *question.Request) {
	if perm != nil {
		if m.heldPerms == nil {
			m.heldPerms = make(map[string]permission.PermissionRequest)
		}
		m.heldPerms[root] = *perm
	}
	if q != nil {
		m.holdQuestionForm(root, m.newQuestionForm(*q))
	}
}

func (m *UI) holdQuestionForm(root string, form *dialog.QuestionForm) {
	if m.heldQuestions == nil {
		m.heldQuestions = make(map[string]*dialog.QuestionForm)
	}
	m.heldQuestions[root] = form
}

// waitingInfo names a waiting session not on screen in the status line.
func (m *UI) waitingInfo(id string) tea.Cmd {
	name := "A background session"
	for _, s := range m.sessionList {
		if s.ID == id {
			name = fmt.Sprintf("Session %q", s.Title)
			break
		}
	}
	return util.ReportInfo(fmt.Sprintf("%s needs your input (%s to open)", name, m.keyMap.Sessions.Help().Key))
}

// parkPrompts holds the prompts open on the session being left, so they come
// back when it is opened again. Call it before m.session changes.
func (m *UI) parkPrompts() {
	id := m.currentSessionID()
	if id == "" {
		return
	}
	if open, ok := m.dialog.Dialog(dialog.PermissionsID).(*dialog.Permissions); ok {
		m.dialog.CloseDialog(dialog.PermissionsID)
		perm := open.Permission()
		m.holdPrompt(id, &perm, nil)
		m.activity.Waiting(id)
	}
	if form, ok := m.activeInline.(*dialog.QuestionForm); ok {
		m.activeInline = nil
		m.textarea.Focus()
		m.holdQuestionForm(id, form)
		m.activity.Waiting(id)
	}
}

// openHeldPrompts shows the prompts held for the session now on screen.
func (m *UI) openHeldPrompts() {
	id := m.currentSessionID()
	if perm, ok := m.heldPerms[id]; ok {
		delete(m.heldPerms, id)
		m.openPrompt(&perm, nil)
	}
	if form, ok := m.heldQuestions[id]; ok {
		delete(m.heldQuestions, id)
		m.showQuestionForm(form)
	}
}

// dropHeldPrompts forgets the prompts held for a session whose run ended:
// they can no longer be answered.
func (m *UI) dropHeldPrompts(id string) {
	delete(m.heldPerms, id)
	delete(m.heldQuestions, id)
}

// dropHeldPermission forgets a held permission resolved elsewhere.
func (m *UI) dropHeldPermission(toolCallID string) {
	for id, perm := range m.heldPerms {
		if perm.ToolCallID == toolCallID {
			delete(m.heldPerms, id)
		}
	}
}

// dropHeldQuestion forgets a held question answered or cancelled elsewhere.
func (m *UI) dropHeldQuestion(batchID string) {
	for id, form := range m.heldQuestions {
		if form.BatchID == batchID {
			delete(m.heldQuestions, id)
		}
	}
}

// probedWaitingInfo names, in the status line, a session not on screen that
// a probe newly reports waiting: in client/server mode the server keeps
// other sessions' prompts from this client, so the probe is how it learns of
// them. Call it before the probes are reconciled.
func (m *UI) probedWaitingInfo(status map[string]workspace.SessionStatus) tea.Cmd {
	for id, st := range status {
		if st.Known && st.PendingPrompt && id != m.currentSessionID() && m.activity.Get(id) != activityWaiting {
			return m.waitingInfo(id)
		}
	}
	return nil
}
