package model

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// promptWS records how permission and question prompts get answered.
type promptWS struct {
	*countingWorkspace
	grants, denies []permission.PermissionRequest
	answers        int
	cancels        int
}

func (w *promptWS) PermissionGrant(p permission.PermissionRequest) bool {
	w.grants = append(w.grants, p)
	return true
}

func (w *promptWS) PermissionGrantPersistent(p permission.PermissionRequest) bool {
	w.grants = append(w.grants, p)
	return true
}

func (w *promptWS) PermissionDeny(p permission.PermissionRequest) bool {
	w.denies = append(w.denies, p)
	return true
}

func (w *promptWS) QuestionAnswer([]question.Answer) bool { w.answers++; return true }
func (w *promptWS) QuestionCancel() bool                  { w.cancels++; return true }

// newPromptUI is a UI on s1 (of s1..s3, listed) with a focused editor.
func newPromptUI(t *testing.T) (*UI, *promptWS) {
	t.Helper()
	ws := &promptWS{countingWorkspace: &countingWorkspace{
		ready:    true,
		sessions: makeSessions(3),
		cfg:      &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}},
	}}
	m := newBusyUI(ws.countingWorkspace)
	m.com = common.DefaultCommon(ws)
	m.status = NewStatus(m.com, nil)
	warmCaches(m, false)
	m.sessionList = ws.sessions
	m.textarea.Focus()
	return m, ws
}

func permEvent(sessionID, id string) pubsub.Event[permission.PermissionRequest] {
	return pubsub.Event[permission.PermissionRequest]{Type: pubsub.CreatedEvent, Payload: permission.PermissionRequest{
		ID: id, SessionID: sessionID, ToolCallID: "tc-" + id, ToolName: "bash", Action: "execute",
	}}
}

func questionEvent(sessionID, id string) pubsub.Event[question.Request] {
	return pubsub.Event[question.Request]{Type: pubsub.CreatedEvent, Payload: question.Request{
		ID: id, SessionID: sessionID,
		Questions: []question.Question{{ID: "q-" + id, Type: question.TypeYesNo, Text: "Go on?", Description: "d"}},
	}}
}

// openPermToolCall returns the tool call ID of the open permission dialog,
// "" when none is open.
func openPermToolCall(m *UI) string {
	if p, ok := m.dialog.Dialog(dialog.PermissionsID).(*dialog.Permissions); ok {
		return p.ToolCallID()
	}
	return ""
}

// openQuestionBatch returns the batch ID of the open question form, "" when
// none is open.
func openQuestionBatch(m *UI) string {
	if f, ok := m.activeInline.(*dialog.QuestionForm); ok {
		return f.BatchID
	}
	return ""
}

// pastDialogGrace waits out the permission dialog's input grace period
// (dialog.graceQuietPeriod, 425ms), after which keys act on it.
func pastDialogGrace() { time.Sleep(500 * time.Millisecond) }

func enter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }

// pending makes the status probe report a prompt pending in these sessions,
// as the workspace does while one is.
func pending(ws *promptWS, ids ...string) {
	ws.sessionStatus = map[string]workspace.SessionStatus{}
	for _, id := range ids {
		ws.sessionStatus[id] = workspace.SessionStatus{PendingPrompt: true}
	}
}

// runPromptCmds is runCmds for the prompt and list results, returning the
// status lines the tree emitted.
func runPromptCmds(m *UI, cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}
	var infos []string
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			infos = append(infos, runPromptCmds(m, c)...)
		}
	case util.InfoMsg:
		infos = append(infos, msg.Msg)
	case sessionListMsg, promptRootMsg:
		_, next := m.Update(msg)
		infos = append(infos, runPromptCmds(m, next)...)
	}
	return infos
}

// refreshInfos runs one session list refresh and returns its status lines.
func refreshInfos(m *UI) []string {
	m.sessionListFetchedAt = time.Time{}
	return runPromptCmds(m, m.dispatchSessionListRefresh())
}

// A permission of a session not on screen opens no dialog: the editor keeps
// focus and the keys, the session shows waiting, and a status line says so.
func TestBackgroundPermissionKeepsEditor(t *testing.T) {
	t.Parallel()

	m, ws := newPromptUI(t)
	pending(ws, "s2")
	_, cmd := m.Update(permEvent("s2", "p2"))
	infos := runPromptCmds(m, cmd)
	typeText(m, "hi")

	require.Equal(t, "hi", m.textarea.Value(), "keys typed in s1 belong to s1's editor")
	require.False(t, m.dialog.HasDialogs())
	require.Equal(t, activityWaiting, m.activity.Get("s2"))
	require.Len(t, infos, 1)
	require.Contains(t, infos[0], "title 2")
	require.Contains(t, infos[0], "alt+s")
}

// Enter and esc in the editor of s1 never answer s2's permission, even after
// the dialog grace period.
func TestEditorKeysDoNotAnswerBackgroundPermission(t *testing.T) {
	t.Parallel()

	m, ws := newPromptUI(t)
	m.Update(permEvent("s2", "p2"))
	pastDialogGrace()
	m.Update(enter())
	m.Update(esc)

	require.Empty(t, ws.grants, "enter in s1 approved s2's request")
	require.Empty(t, ws.denies, "esc in s1 denied s2's request")
}

// A question of a session not on screen does not take the editor; esc and
// enter in s1 neither cancel nor answer it.
func TestBackgroundQuestionKeepsEditor(t *testing.T) {
	t.Parallel()

	m, ws := newPromptUI(t)
	m.Update(questionEvent("s2", "q2"))
	m.Update(esc)
	m.Update(enter())

	require.Zero(t, ws.cancels, "esc in s1 cancelled s2's question")
	require.Zero(t, ws.answers, "enter in s1 answered s2's question")
	require.Empty(t, openQuestionBatch(m))
	typeText(m, "hi")
	require.Equal(t, "hi", m.textarea.Value())
	require.Equal(t, activityWaiting, m.activity.Get("s2"))
}

// An open picker stays in front when a background session asks.
func TestBackgroundPromptLeavesPickerOpen(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.handleKeyPressMsg(altS)
	require.True(t, m.dialog.ContainsDialog(dialog.SessionsID))

	m.Update(permEvent("s2", "p2"))
	require.False(t, m.dialog.ContainsDialog(dialog.PermissionsID))
	require.Equal(t, dialog.SessionsID, m.dialog.DialogLast().ID())
}

// The held permission opens when its session is opened, and answering it
// answers exactly that request.
func TestHeldPermissionOpensOnSwitch(t *testing.T) {
	t.Parallel()

	m, ws := newPromptUI(t)
	m.Update(permEvent("s2", "p2"))
	switchTo(m, "s3")
	require.Empty(t, openPermToolCall(m), "s3 has nothing to answer")

	switchTo(m, "s2")
	require.Equal(t, "tc-p2", openPermToolCall(m))
	pastDialogGrace()
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	require.Len(t, ws.grants, 1)
	require.Equal(t, "p2", ws.grants[0].ID)
	require.Equal(t, "s2", ws.grants[0].SessionID)
	require.Empty(t, ws.denies)
	require.False(t, m.dialog.HasDialogs())
}

// Prompts of the session on screen open at once, as before.
func TestCurrentSessionPromptsOpenImmediately(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(permEvent("s1", "p1"))
	require.Equal(t, "tc-p1", openPermToolCall(m))

	m.Update(questionEvent("s1", "q1"))
	require.Equal(t, "q1", openQuestionBatch(m))
}

// Two background sessions waiting: each prompt shows only in its own
// session, and leaving a session parks its prompt until it is reopened.
func TestTwoBackgroundSessionsEachShownInOwnSession(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(permEvent("s2", "p2"))
	m.Update(questionEvent("s3", "q3"))
	require.False(t, m.dialog.HasDialogs())
	require.Empty(t, openQuestionBatch(m))
	require.Equal(t, activityWaiting, m.activity.Get("s2"))
	require.Equal(t, activityWaiting, m.activity.Get("s3"))

	switchTo(m, "s3")
	require.Equal(t, "q3", openQuestionBatch(m))
	require.Empty(t, openPermToolCall(m))

	switchTo(m, "s2")
	require.Equal(t, "tc-p2", openPermToolCall(m))
	require.Empty(t, openQuestionBatch(m), "s3's question left with s3")

	switchTo(m, "s3")
	require.Empty(t, openPermToolCall(m), "s2's permission left with s2")
	require.Equal(t, "q3", openQuestionBatch(m))

	switchTo(m, "s2")
	require.Equal(t, "tc-p2", openPermToolCall(m))
}

// A sub-agent's prompt belongs to its top-level session.
func TestChildSessionPromptMapsToRoot(t *testing.T) {
	t.Parallel()

	m, ws := newPromptUI(t)
	own, other := childSession("s1", "tc1"), childSession("s2", "tc2")
	ws.children = []session.Session{own, other}
	pending(ws, "s2")

	_, cmd := m.Update(permEvent(other.ID, "pc2"))
	runCmds(m, cmd)
	require.False(t, m.dialog.HasDialogs(), "a child of s2 is not on screen in s1")
	require.Equal(t, activityWaiting, m.activity.Get("s2"))

	_, cmd = m.Update(permEvent(own.ID, "pc1"))
	runCmds(m, cmd)
	require.Equal(t, "tc-pc1", openPermToolCall(m), "a child of s1 is on screen")

	m.dialog.CloseDialog(dialog.PermissionsID)
	switchTo(m, "s2")
	require.Equal(t, "tc-pc2", openPermToolCall(m))
}

// A prompt resolved elsewhere, or whose run ended, is not shown later.
func TestResolvedHeldPromptsAreDropped(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(permEvent("s2", "p2"))
	m.Update(pubsub.Event[permission.PermissionNotification]{Payload: permission.PermissionNotification{
		SessionID: "s2", ToolCallID: "tc-p2", Granted: true,
	}})
	m.Update(questionEvent("s3", "q3"))
	m.Update(pubsub.Event[notify.RunComplete]{Payload: notify.RunComplete{SessionID: "s3", Cancelled: true, Error: "context canceled"}})

	switchTo(m, "s2")
	require.False(t, m.dialog.HasDialogs())
	switchTo(m, "s3")
	require.Empty(t, openQuestionBatch(m))
}

// A late notification for another batch must not close the open form.
func TestLateQuestionNotificationKeepsOtherForm(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(questionEvent("s1", "qB"))
	require.Equal(t, "qB", openQuestionBatch(m))

	m.Update(pubsub.Event[question.Notification]{Payload: question.Notification{BatchID: "qA", SessionID: "s1"}})
	require.Equal(t, "qB", openQuestionBatch(m))

	m.Update(pubsub.Event[question.Notification]{Payload: question.Notification{BatchID: "qB", SessionID: "s1"}})
	require.Empty(t, openQuestionBatch(m))
}

// In client/server mode the server filters other sessions' prompts out; the
// status probe is what reports them, and it raises the same status line.
func TestProbedBackgroundPromptShowsInfo(t *testing.T) {
	t.Parallel()

	m, ws := newPromptUI(t)
	pending(ws, "s1")
	require.Empty(t, refreshInfos(m), "the session on screen shows its own prompt")

	pending(ws, "s3")
	infos := refreshInfos(m)
	require.Equal(t, activityWaiting, m.activity.Get("s3"))
	require.Len(t, infos, 1)
	require.Contains(t, infos[0], "title 3")
	require.Empty(t, refreshInfos(m), "said once, not on every probe")
}

// A client that has not opened a session yet (landing screen) gets every
// prompt from the server, which fails open for it: nothing opens over the
// editor until the session is opened.
func TestLandingHoldsPrompts(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.session = nil
	m.setState(uiLanding, uiFocusEditor)
	m.Update(permEvent("s2", "p2"))
	require.False(t, m.dialog.HasDialogs())
	require.Equal(t, activityWaiting, m.activity.Get("s2"))

	switchTo(m, "s2")
	require.Equal(t, "tc-p2", openPermToolCall(m))
}
