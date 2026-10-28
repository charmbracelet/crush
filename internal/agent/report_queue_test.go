package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// A report that arrives while the parent is busy waits in the queue. If
// that turn is then cancelled, the report must still reach the transcript;
// otherwise the subagent reads as never having reported, and the next
// launch tells the parent it was interrupted.
func TestCancelKeepsQueuedSubagentReport(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	report := message.SubAgentReport{Label: "betty-review", Output: "all good"}
	a.messageQueue.Set(parent.ID, []SessionAgentCall{
		{SessionID: parent.ID, Prompt: "follow-up the user typed", acceptSeq: 1},
		{SessionID: parent.ID, Prompt: reportPrompt(report), PromptParts: []message.ContentPart{report}},
	})

	a.Cancel(parent.ID)

	msgs, err := env.messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	var reports []message.SubAgentReport
	for _, m := range msgs {
		reports = append(reports, m.SubAgentReports()...)
	}
	require.Equal(t, []message.SubAgentReport{report}, reports)
}

// A report queued behind a cancel is folded into the step rather than
// dropped with the prompts the cancel covers.
func TestDrainKeepsReportCoveredByCancel(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "drain-report"
	report := message.SubAgentReport{Label: "betty-review", Output: "all good"}
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "withdrawn", acceptSeq: 1},
		{SessionID: sessionID, Prompt: reportPrompt(report), PromptParts: []message.ContentPart{report}},
	})
	a.cancelMark.Set(sessionID, 1)

	fold, _ := a.drainQueueForStep(sessionID)
	require.Len(t, fold, 1)
	require.True(t, carriesReport(fold[0]))
}
