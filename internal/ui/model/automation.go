package model

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/automation"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// automationState holds the goal and loop state for the open session. The
// state is persisted per session so a resumed conversation keeps its goal
// and its scheduled tasks.
type automationState struct {
	store     *automation.Store
	state     automation.State
	sessionID string

	// pendingGoal is true while a goal turn is in flight.
	pendingGoal bool
	// pendingLoopID is the loop whose iteration is in flight.
	pendingLoopID string
	// verifying is true while the independent completion check runs.
	verifying bool
	// tickScheduled prevents duplicate scheduler timers.
	tickScheduled bool
	// disabled is set by the kill switch environment variable.
	disabled bool
}

// automationTickMsg fires the loop scheduler.
type automationTickMsg struct{}

// goalVerifiedMsg carries the independent completion check result.
type goalVerifiedMsg struct {
	evidence string
	verdict  automation.Verdict
	err      error
}

func newAutomationState(com *common.Common) *automationState {
	dir := ""
	if cfg := com.Config(); cfg != nil && cfg.Options != nil {
		dir = cfg.Options.DataDirectory
		if dir != "" && !filepath.IsAbs(dir) {
			dir = filepath.Join(com.Workspace.WorkingDir(), dir)
		}
	}
	if dir == "" {
		dir = filepath.Join(home.Dir(), ".crush")
	}
	return &automationState{
		store:    automation.NewStore(filepath.Join(dir, "automation")),
		disabled: os.Getenv("CRUSH_DISABLE_CRON") == "1" || os.Getenv("COMMANDCODE_DISABLE_CRON") == "1",
	}
}

// load reads the automation state for a session. An active goal is marked
// idle because a resumed session does not restart its loop.
func (a *automationState) load(sessionID string) {
	if a == nil {
		return
	}
	a.sessionID = sessionID
	a.state = automation.State{}
	a.pendingGoal = false
	a.pendingLoopID = ""
	a.verifying = false
	if a.store == nil || sessionID == "" {
		return
	}
	state, err := a.store.Load(sessionID)
	if err != nil {
		slog.Error("Failed to load automation state", "error", err, "session_id", sessionID)
		return
	}
	a.state = state
	if a.state.Goal != nil && a.state.Goal.Status == automation.GoalActive {
		a.state.Goal.Status = automation.GoalIdle
		a.save()
	}
	// A self-paced loop that was mid-iteration when the session closed
	// comes back as a pending safety check rather than firing at once.
	now := time.Now()
	changed := false
	for i := range a.state.Loops {
		if a.state.Loops[i].Kind == automation.LoopSelfPaced && a.state.Loops[i].NextRun.Before(now) {
			a.state.Loops[i].NextRun = now.Add(automation.LoopSafetyDelay)
			changed = true
		}
	}
	if changed {
		a.save()
	}
}

// ensureLoaded loads the session's state when the open session changed.
func (a *automationState) ensureLoaded(sessionID string) {
	if a == nil || a.sessionID == sessionID {
		return
	}
	a.load(sessionID)
}

func (a *automationState) save() {
	if a == nil || a.store == nil || a.sessionID == "" {
		return
	}
	if err := a.store.Save(a.sessionID, a.state); err != nil {
		slog.Error("Failed to save automation state", "error", err, "session_id", a.sessionID)
	}
}

func (a *automationState) goalStatus() string {
	if a == nil {
		return "No goal set"
	}
	return a.state.Goal.StatusLine(time.Now())
}

func (a *automationState) loopStatus() string {
	if a == nil || len(a.state.Loops) == 0 {
		return "No loops scheduled"
	}
	now := time.Now()
	parts := make([]string, 0, len(a.state.Loops))
	for _, task := range a.state.Loops {
		cadence := "self-paced"
		if task.Kind == automation.LoopFixed {
			cadence = "every " + automation.FormatDuration(task.Interval)
		}
		next := task.NextRun.Sub(now)
		next = max(next, 0)
		parts = append(parts, fmt.Sprintf("%s · %s · next in %s", task.ID, cadence, automation.FormatDuration(next)))
	}
	return "Loops: " + strings.Join(parts, "; ")
}

// handleGoalCommand implements /goal and its subcommands.
func (m *UI) handleGoalCommand(args string) tea.Cmd {
	if m.automation == nil {
		return util.ReportError(errors.New("automation is not available"))
	}
	args = strings.TrimSpace(args)
	switch args {
	case "":
		goal := m.automation.state.Goal
		if goal != nil && (goal.Status == automation.GoalIdle || goal.Status == automation.GoalPaused) {
			return m.resumeGoal()
		}
		return util.ReportInfo(m.automation.goalStatus())
	case "status":
		return util.ReportInfo(m.automation.goalStatus())
	case "clear":
		return m.clearGoal()
	}
	objective, budget := parseGoalArgs(args)
	if objective == "" {
		return util.ReportInfo(m.automation.goalStatus())
	}
	return m.startGoal(objective, budget)
}

func parseGoalArgs(args string) (string, int) {
	budget := 0
	fields := strings.Fields(args)
	var kept []string
	for i := 0; i < len(fields); i++ {
		switch {
		case fields[i] == "--max-turns" && i+1 < len(fields):
			if n, err := parsePositiveInt(fields[i+1]); err == nil {
				budget = n
			}
			i++
		case strings.HasPrefix(fields[i], "--max-turns="):
			if n, err := parsePositiveInt(strings.TrimPrefix(fields[i], "--max-turns=")); err == nil {
				budget = n
			}
		default:
			kept = append(kept, fields[i])
		}
	}
	return strings.Join(kept, " "), budget
}

func parsePositiveInt(s string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number %q", s)
	}
	return n, nil
}

func (m *UI) startGoal(objective string, budget int) tea.Cmd {
	loadCmd, err := m.ensureSession()
	if err != nil {
		return util.ReportError(err)
	}
	if m.session == nil {
		return util.ReportError(errors.New("no session to attach the goal to"))
	}
	m.automation.ensureLoaded(m.session.ID)
	goal := automation.NewGoal(objective, budget, time.Now())
	m.automation.state.Goal = goal
	m.automation.pendingGoal = true
	m.automation.verifying = false
	m.automation.save()

	cmds := []tea.Cmd{util.ReportInfo("Goal set: " + truncateText(objective, 60))}
	if loadCmd != nil {
		cmds = append(cmds, loadCmd)
	}
	cmds = append(cmds, m.sendMessageInternal(automation.GoalPrompt(goal.Objective, 0, goal.Budget, ""), true))
	return tea.Batch(cmds...)
}

func (m *UI) resumeGoal() tea.Cmd {
	goal := m.automation.state.Goal
	if goal == nil {
		return util.ReportInfo("No goal set")
	}
	if goal.Status == automation.GoalActive {
		return util.ReportInfo(goal.StatusLine(time.Now()))
	}
	goal.Resume()
	m.automation.pendingGoal = true
	m.automation.verifying = false
	m.automation.save()
	return tea.Batch(
		util.ReportInfo("Goal resumed"),
		m.sendMessageInternal(automation.GoalPrompt(goal.Objective, goal.Turn, goal.Budget, goal.LastVerification), true),
	)
}

func (m *UI) clearGoal() tea.Cmd {
	if m.automation.state.Goal == nil {
		return util.ReportInfo("No goal set")
	}
	m.automation.state.Goal = nil
	m.automation.pendingGoal = false
	m.automation.verifying = false
	m.automation.save()
	return util.ReportInfo("Goal cleared")
}

// handleLoopCommand implements /loop and its subcommands.
func (m *UI) handleLoopCommand(args string) tea.Cmd {
	if m.automation == nil {
		return util.ReportError(errors.New("automation is not available"))
	}
	if m.automation.disabled {
		return util.ReportInfo("Loops are disabled (CRUSH_DISABLE_CRON=1)")
	}
	if m.mode == uiInputModePlan {
		return util.ReportInfo("Loops cannot start in plan mode; leave plan mode and run /loop again")
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "status":
		return util.ReportInfo(m.automation.loopStatus())
	case args == "clear":
		return m.clearLoops()
	case strings.HasPrefix(args, "cancel "):
		return m.cancelLoop(strings.TrimSpace(strings.TrimPrefix(args, "cancel ")))
	}
	defaultTask := automation.DefaultTask(
		m.com.Workspace.WorkingDir(),
		filepath.Join(home.Config(), "crush", "loop.md"),
	)
	req, err := automation.ParseLoopRequest(args, defaultTask)
	if err != nil {
		return util.ReportError(err)
	}
	return m.startLoop(req)
}

func (m *UI) startLoop(req automation.LoopRequest) tea.Cmd {
	loadCmd, err := m.ensureSession()
	if err != nil {
		return util.ReportError(err)
	}
	if m.session == nil {
		return util.ReportError(errors.New("no session to attach the loop to"))
	}
	m.automation.ensureLoaded(m.session.ID)
	if len(m.automation.state.Loops) >= automation.MaxLoopsPerSession {
		return util.ReportError(fmt.Errorf("at most %d loops are allowed per conversation", automation.MaxLoopsPerSession))
	}
	if !req.Fixed {
		m.automation.state.Loops = slices.DeleteFunc(m.automation.state.Loops, func(task automation.Task) bool {
			return task.Kind == automation.LoopSelfPaced
		})
	}

	kind := automation.LoopSelfPaced
	if req.Fixed {
		kind = automation.LoopFixed
	}
	now := time.Now()
	task := automation.NewTask(automation.NewTaskID(), req.Task, kind, req.Interval, now)
	task.Default = req.Default
	m.automation.state.Loops = append(m.automation.state.Loops, task)
	m.automation.save()

	cmds := []tea.Cmd{util.ReportInfo(loopStartedMessage(task))}
	if loadCmd != nil {
		cmds = append(cmds, loadCmd)
	}
	if m.isAgentBusy() {
		// The scheduler retries once the current turn finishes.
		task.NextRun = now.Add(5 * time.Second)
		m.automation.state.Loops[len(m.automation.state.Loops)-1] = task
		m.automation.save()
		return tea.Batch(append(cmds, m.scheduleAutomationTick())...)
	}
	idx := len(m.automation.state.Loops) - 1
	task.LastRun = now
	m.automation.state.Loops[idx] = task
	m.automation.pendingLoopID = task.ID
	m.automation.save()
	cmds = append(cmds, m.sendMessageInternal(automation.LoopPrompt(task, 0), true))
	cmds = append(cmds, m.scheduleAutomationTick())
	return tea.Batch(cmds...)
}

func loopStartedMessage(task automation.Task) string {
	if task.Kind == automation.LoopSelfPaced {
		return "Loop started · " + task.ID + " · self-paced"
	}
	return "Loop started · " + task.ID + " · every " + automation.FormatDuration(task.Interval)
}

func (m *UI) clearLoops() tea.Cmd {
	n := len(m.automation.state.Loops)
	if n == 0 {
		return util.ReportInfo("No loops scheduled")
	}
	m.automation.state.Loops = nil
	m.automation.pendingLoopID = ""
	m.automation.save()
	return util.ReportInfo(fmt.Sprintf("Loop stopped · %d scheduled tasks cancelled", n))
}

func (m *UI) cancelLoop(id string) tea.Cmd {
	if id == "" {
		return util.ReportError(errors.New("usage: /loop cancel <id>"))
	}
	if !m.automation.state.RemoveLoop(id) {
		return util.ReportInfo("No loop with id " + id)
	}
	m.automation.pendingLoopID = ""
	m.automation.save()
	return util.ReportInfo("Loop stopped · " + id)
}

// cancelAutomationLoops cancels every loop in the conversation. It is the
// Esc action and leaves goals alone.
func (m *UI) cancelAutomationLoops() tea.Cmd {
	if m.automation == nil || len(m.automation.state.Loops) == 0 {
		return nil
	}
	n := len(m.automation.state.Loops)
	m.automation.state.Loops = nil
	m.automation.pendingLoopID = ""
	m.automation.save()
	return util.ReportInfo(fmt.Sprintf("Loop stopped · %d scheduled tasks cancelled", n))
}

func (m *UI) scheduleAutomationTick() tea.Cmd {
	if m.automation == nil || m.automation.tickScheduled || len(m.automation.state.Loops) == 0 {
		return nil
	}
	m.automation.tickScheduled = true
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return automationTickMsg{}
	})
}

func (m *UI) handleAutomationTick() tea.Cmd {
	if m.automation == nil || !m.hasSession() {
		return nil
	}
	now := time.Now()
	expired := m.automation.state.RemoveExpiredLoops(now)
	var cmds []tea.Cmd
	if len(expired) > 0 {
		m.automation.save()
		cmds = append(cmds, util.ReportInfo(fmt.Sprintf("%d loop(s) expired", len(expired))))
	}
	if len(m.automation.state.Loops) == 0 {
		return tea.Batch(cmds...)
	}
	if m.isAgentBusy() || m.automation.pendingGoal || m.automation.pendingLoopID != "" || m.automation.verifying {
		return tea.Batch(append(cmds, m.scheduleAutomationTick())...)
	}

	idx := -1
	for i := range m.automation.state.Loops {
		if !m.automation.state.Loops[i].NextRun.After(now) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return tea.Batch(append(cmds, m.scheduleAutomationTick())...)
	}

	task := m.automation.state.Loops[idx]
	task.LastRun = now
	m.automation.state.Loops[idx] = task
	m.automation.pendingLoopID = task.ID
	m.automation.save()
	cmds = append(cmds, m.sendMessageInternal(automation.LoopPrompt(task, task.QuietStreak), true))
	cmds = append(cmds, m.scheduleAutomationTick())
	return tea.Batch(cmds...)
}

// handleAutomationRunComplete advances the goal or loop whose turn just
// finished.
func (m *UI) handleAutomationRunComplete(rc notify.RunComplete) tea.Cmd {
	if m.automation == nil || !m.hasSession() || rc.SessionID != m.session.ID {
		return nil
	}
	if m.automation.pendingLoopID != "" {
		return m.handleLoopRunComplete(rc)
	}
	if m.automation.state.Goal != nil && m.automation.state.Goal.Status == automation.GoalActive && !m.automation.verifying {
		return m.handleGoalRunComplete(rc)
	}
	return nil
}

func (m *UI) handleGoalRunComplete(rc notify.RunComplete) tea.Cmd {
	goal := m.automation.state.Goal
	m.automation.pendingGoal = false
	if rc.Cancelled || rc.Error != "" {
		reason := "run cancelled"
		if rc.Error != "" {
			reason = "run failed"
		}
		goal.Pause(reason, time.Now())
		m.automation.save()
		return util.ReportInfo(goal.StatusLine(time.Now()))
	}
	steps := rc.Steps
	steps = max(steps, 1)
	budgetRemains := goal.AddTurns(steps)
	// A completion claim on the last budgeted turn still gets checked;
	// only an unfinished goal pauses when the budget runs out.
	if evidence, ok := automation.ParseGoalComplete(rc.Text); ok {
		m.automation.verifying = true
		m.automation.save()
		return m.verifyGoal(goal.Objective, evidence)
	}
	if !budgetRemains {
		goal.Pause("out of turns", time.Now())
		m.automation.save()
		return util.ReportInfo(goal.StatusLine(time.Now()))
	}
	m.automation.pendingGoal = true
	m.automation.save()
	return m.sendMessageInternal(automation.GoalPrompt(goal.Objective, goal.Turn, goal.Budget, goal.LastVerification), true)
}

func (m *UI) verifyGoal(objective, evidence string) tea.Cmd {
	sessionID := m.session.ID
	prompt := automation.VerificationPrompt(objective, evidence)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		answer, err := m.com.Workspace.AgentAsk(ctx, sessionID, prompt)
		if err != nil {
			return goalVerifiedMsg{evidence: evidence, err: err}
		}
		return goalVerifiedMsg{evidence: evidence, verdict: automation.ParseVerdict(answer)}
	}
}

func (m *UI) handleGoalVerified(msg goalVerifiedMsg) tea.Cmd {
	goal := m.automation.state.Goal
	m.automation.verifying = false
	if goal == nil || goal.Status != automation.GoalActive {
		return nil
	}
	if msg.err != nil {
		msg.verdict = automation.Verdict{Verified: false, Reason: msg.err.Error()}
	}
	if msg.verdict.Verified {
		goal.Achieve(time.Now())
		m.automation.save()
		return util.ReportInfo(goal.StatusLine(time.Now()))
	}
	goal.VerificationFailures++
	goal.LastVerification = msg.verdict.Reason
	if goal.VerificationFailures >= automation.MaxGoalVerificationFailures {
		goal.Pause("couldn't verify completion", time.Now())
		m.automation.save()
		return util.ReportInfo(goal.StatusLine(time.Now()))
	}
	if !goal.AddTurns(1) {
		goal.Pause("out of turns", time.Now())
		m.automation.save()
		return util.ReportInfo(goal.StatusLine(time.Now()))
	}
	m.automation.pendingGoal = true
	m.automation.save()
	return m.sendMessageInternal(automation.GoalPrompt(goal.Objective, goal.Turn, goal.Budget, goal.LastVerification), true)
}

func (m *UI) handleLoopRunComplete(rc notify.RunComplete) tea.Cmd {
	id := m.automation.pendingLoopID
	m.automation.pendingLoopID = ""
	idx := m.automation.state.FindLoop(id)
	if idx < 0 {
		return nil
	}
	task := m.automation.state.Loops[idx]
	now := time.Now()
	if rc.Cancelled || rc.Error != "" {
		if task.Kind == automation.LoopFixed {
			task.NextRun = automation.NextFixed(now, task)
		} else {
			task.NextRun = now.Add(automation.LoopSafetyDelay)
		}
		m.automation.state.Loops[idx] = task
		m.automation.save()
		return m.scheduleAutomationTick()
	}

	output := strings.TrimSpace(rc.Text)
	if output == task.LastOutput {
		task.QuietStreak++
	} else {
		task.QuietStreak = 0
	}
	task.LastOutput = output

	if task.Kind == automation.LoopSelfPaced {
		if report, ok := automation.ParseLoopDone(rc.Text); ok {
			m.automation.state.RemoveLoop(id)
			m.automation.save()
			return util.ReportInfo("Loop finished: " + truncateText(report, 80))
		}
		if delay, _, ok := automation.ParseLoopWakeup(rc.Text); ok {
			task.NextRun = now.Add(delay)
			task.SafetyChecks = 0
		} else {
			task.SafetyChecks++
			if task.SafetyChecks >= 2 {
				m.automation.state.RemoveLoop(id)
				m.automation.save()
				return util.ReportInfo("Loop stopped · no next check scheduled")
			}
			task.NextRun = now.Add(automation.LoopSafetyDelay)
		}
	} else {
		task.NextRun = automation.NextFixed(now, task)
	}
	m.automation.state.Loops[idx] = task
	m.automation.save()
	return m.scheduleAutomationTick()
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
