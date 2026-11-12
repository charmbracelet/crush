package model

import (
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/stretchr/testify/require"
)

// questionWorkspace is a countingWorkspace that records the answers the
// question form submits.
type questionWorkspace struct {
	*countingWorkspace
	answers [][]question.Answer
}

func (w *questionWorkspace) QuestionAnswer(responses []question.Answer) bool {
	w.answers = append(w.answers, responses)
	return true
}

// recordingNotifier is a notification backend that records what it is
// asked to send.
type recordingNotifier struct {
	sent []notification.Notification
}

func (r *recordingNotifier) Send(n notification.Notification) tea.Cmd {
	r.sent = append(r.sent, n)
	return nil
}

// newQuestionUI builds a UI that runs question events through Update with
// CRUSH_QUESTIONS_PIPED set to piped, so a value inherited from the
// environment cannot leak in. The chat starts scrolled to the top and the
// window unfocused, so the scroll and the notification a question triggers
// can be observed.
func newQuestionUI(t *testing.T, piped string) (*UI, *questionWorkspace, *recordingNotifier) {
	t.Helper()
	t.Setenv("CRUSH_QUESTIONS_PIPED", piped)

	u := newProgramStatusUI(t)
	ws := &questionWorkspace{countingWorkspace: u.com.Workspace.(*countingWorkspace)}
	u.com.Workspace = ws
	notifier := &recordingNotifier{}
	u.notifyBackend = notifier
	u.caps.ReportFocusEvents = true
	u.notifyWindowFocused = false

	msgs := make([]chat.MessageItem, 0, 60)
	for i := range 60 {
		msgs = append(msgs, testMessageItem{
			id:   "m-" + strconv.Itoa(i),
			text: "message " + strconv.Itoa(i),
		})
	}
	u.chat.SetMessages(msgs...)
	u.updateLayoutAndSize()
	u.chat.ScrollToTop()
	require.False(t, u.chat.AtBottom())
	return u, ws, notifier
}

// questionEvent is the event the workspace publishes when the agent asks
// the batch with the given ID, and again whenever the batch is replayed.
func questionEvent(id string) pubsub.Event[question.Request] {
	return pubsub.Event[question.Request]{
		Type: pubsub.CreatedEvent,
		Payload: question.Request{
			ID:        id,
			SessionID: "s1",
			Questions: []question.Question{{
				ID:          id + "-q1",
				Type:        question.TypeFreeText,
				Text:        "Anything else?",
				Description: "Free-form notes.",
			}},
		},
	}
}

func TestQuestionRequestOpensForm(t *testing.T) {
	u, _, notifier := newQuestionUI(t, "")

	_, _ = u.Update(questionEvent("b1"))

	require.IsType(t, &dialog.QuestionForm{}, u.activeInline)
	require.True(t, u.hasPendingQuestion())
	require.True(t, u.chat.AtBottom())
	require.Len(t, notifier.sent, 1)
}

func TestQuestionRequestPiped(t *testing.T) {
	u, _, notifier := newQuestionUI(t, "1")

	_, _ = u.Update(questionEvent("b1"))

	require.Nil(t, u.activeInline)
	require.False(t, u.hasPendingQuestion())
	require.False(t, u.chat.AtBottom())
	require.Empty(t, notifier.sent)
}

func TestQuestionReplayKeepsForm(t *testing.T) {
	u, ws, notifier := newQuestionUI(t, "")

	_, _ = u.Update(questionEvent("b1"))
	form := u.activeInline
	_, _ = u.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	u.chat.ScrollToTop()

	// The server re-sends the pending batch whenever a client reports its
	// session; the replay must not wipe what was typed, nor scroll the chat
	// or notify again.
	_, _ = u.Update(questionEvent("b1"))
	require.Same(t, form, u.activeInline)
	require.False(t, u.chat.AtBottom())
	require.Len(t, notifier.sent, 1)

	_, _ = u.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, u.activeInline)
	require.Equal(t, [][]question.Answer{{{QuestionID: "b1-q1", FillInText: "x"}}}, ws.answers)
}

func TestQuestionNewBatchReplacesForm(t *testing.T) {
	u, _, _ := newQuestionUI(t, "")

	_, _ = u.Update(questionEvent("b1"))
	form := u.activeInline

	_, _ = u.Update(questionEvent("b2"))
	require.NotSame(t, form, u.activeInline)
	require.Equal(t, "b2", u.activeInline.(*dialog.QuestionForm).BatchID)
}
