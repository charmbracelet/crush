package model

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/automation"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/discord"
	"github.com/stretchr/testify/require"
)

func newAutomationUI(t *testing.T) (*UI, *slashWorkspace) {
	t.Helper()
	ws := &slashWorkspace{cfg: &config.Config{Options: &config.Options{DataDirectory: t.TempDir()}}}
	m := newSlashUI(ws)
	m.automation = newAutomationState(m.com)
	return m, ws
}

func TestGoalCommandStartsGoal(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	submitEditor(t, m, "/goal get the tests green")

	require.Len(t, ws.prompts, 1)
	require.Contains(t, ws.prompts[0], "get the tests green")
	require.Contains(t, ws.prompts[0], "<goal_complete>")
	require.NotNil(t, m.automation.state.Goal)
	require.Equal(t, automation.GoalActive, m.automation.state.Goal.Status)
	require.True(t, m.automation.pendingGoal)
}

func TestGoalStatusAndClear(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	submitEditor(t, m, "/goal get the tests green")
	before := len(ws.prompts)

	submitEditor(t, m, "/goal status")
	require.Len(t, ws.prompts, before, "/goal status must not send a prompt")

	submitEditor(t, m, "/goal clear")
	require.Nil(t, m.automation.state.Goal)
	require.False(t, m.automation.pendingGoal)
}

func TestGoalRunCompleteContinues(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	submitEditor(t, m, "/goal get the tests green")

	cmd := m.handleAutomationRunComplete(notify.RunComplete{
		SessionID: "s1",
		Text:      "still working",
		Steps:     1,
	})
	runCmd(cmd)

	require.Len(t, ws.prompts, 2)
	require.Equal(t, 1, m.automation.state.Goal.Turn)
	require.True(t, m.automation.pendingGoal)
}

func TestGoalCompletionIsVerified(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	ws.askAnswer = `{"verified": true, "reason": "tests pass"}`
	submitEditor(t, m, "/goal get the tests green")

	cmd := m.handleAutomationRunComplete(notify.RunComplete{
		SessionID: "s1",
		Text:      "done <goal_complete>tests pass</goal_complete>",
		Steps:     1,
	})
	require.NotNil(t, cmd)
	msg, ok := cmd().(goalVerifiedMsg)
	require.True(t, ok)
	require.True(t, msg.verdict.Verified)

	m.handleGoalVerified(msg)
	require.Equal(t, automation.GoalAchieved, m.automation.state.Goal.Status)
	require.Len(t, ws.prompts, 1)
}

func TestGoalRejectedContinuesWithReason(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	ws.askAnswer = `{"verified": false, "reason": "no test output"}`
	submitEditor(t, m, "/goal get the tests green")

	cmd := m.handleAutomationRunComplete(notify.RunComplete{
		SessionID: "s1",
		Text:      "done <goal_complete>I think it works</goal_complete>",
		Steps:     1,
	})
	msg, ok := cmd().(goalVerifiedMsg)
	require.True(t, ok)
	require.False(t, msg.verdict.Verified)

	runCmd(m.handleGoalVerified(msg))
	require.Len(t, ws.prompts, 2)
	require.Contains(t, ws.prompts[1], "no test output")
	require.True(t, m.automation.pendingGoal)
}

func TestLoopCommandStartsFixedLoop(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	submitEditor(t, m, "/loop 5m check the deploy")

	require.Len(t, ws.prompts, 1)
	require.Contains(t, ws.prompts[0], "check the deploy")
	require.Contains(t, ws.prompts[0], "<loop_task")
	require.Len(t, m.automation.state.Loops, 1)
	task := m.automation.state.Loops[0]
	require.Equal(t, automation.LoopFixed, task.Kind)
	require.Equal(t, 5*time.Minute, task.Interval)
	require.Equal(t, task.ID, m.automation.pendingLoopID)
}

func TestLoopSelfPacedReschedules(t *testing.T) {
	t.Parallel()

	m, _ := newAutomationUI(t)
	submitEditor(t, m, "/loop check the deploy")
	require.Len(t, m.automation.state.Loops, 1)
	require.Equal(t, automation.LoopSelfPaced, m.automation.state.Loops[0].Kind)

	m.handleAutomationRunComplete(notify.RunComplete{
		SessionID: "s1",
		Text:      `still rolling <loop_wakeup delay="5m">watching</loop_wakeup>`,
	})
	require.Empty(t, m.automation.pendingLoopID)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), m.automation.state.Loops[0].NextRun, 10*time.Second)
}

func TestLoopSelfPacedStops(t *testing.T) {
	t.Parallel()

	m, _ := newAutomationUI(t)
	submitEditor(t, m, "/loop check the deploy")

	cmd := m.handleAutomationRunComplete(notify.RunComplete{
		SessionID: "s1",
		Text:      "<loop_done>the deploy is healthy</loop_done>",
	})
	require.NotNil(t, cmd)
	require.Empty(t, m.automation.state.Loops)
}

func TestCancelAutomationLoops(t *testing.T) {
	t.Parallel()

	m, _ := newAutomationUI(t)
	submitEditor(t, m, "/loop 5m check the deploy")
	require.Len(t, m.automation.state.Loops, 1)

	cmd := m.cancelAutomationLoops()
	require.NotNil(t, cmd)
	require.Empty(t, m.automation.state.Loops)
}

func TestLoadSessionKeepsPendingAutomation(t *testing.T) {
	t.Parallel()

	m, _ := newAutomationUI(t)
	submitEditor(t, m, "/goal get the tests green")
	require.True(t, m.automation.pendingGoal)

	m.Update(loadSessionMsg{session: m.session})
	require.True(t, m.automation.pendingGoal)
	require.Equal(t, automation.GoalActive, m.automation.state.Goal.Status)
}

func TestAutomationCommandFromDiscord(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	reply := make(chan string, 1)
	runCmd(m.handleAutomationCommand(discord.AutomationCommandMsg{
		SessionID: "s1",
		Name:      "goal",
		Args:      "get the tests green",
		Reply:     reply,
	}))

	require.Len(t, ws.prompts, 1)
	require.Contains(t, ws.prompts[0], "get the tests green")
	require.Equal(t, "Goal set: get the tests green", <-reply)
	require.True(t, m.automation.pendingGoal)
}

func TestAutomationCommandStatusFromDiscord(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	reply := make(chan string, 1)
	cmd := m.handleAutomationCommand(discord.AutomationCommandMsg{
		SessionID: "s1",
		Name:      "loop",
		Args:      "status",
		Reply:     reply,
	})
	require.Nil(t, cmd)
	require.Empty(t, ws.prompts)
	require.Equal(t, "No loops scheduled", <-reply)
}

func TestAutomationCommandRejectsOtherSession(t *testing.T) {
	t.Parallel()

	m, ws := newAutomationUI(t)
	reply := make(chan string, 1)
	cmd := m.handleAutomationCommand(discord.AutomationCommandMsg{
		SessionID: "other",
		Name:      "goal",
		Args:      "get the tests green",
		Reply:     reply,
	})
	require.Nil(t, cmd)
	require.Empty(t, ws.prompts)
	require.Contains(t, <-reply, "different session")
}
