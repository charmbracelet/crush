package dialog

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// WorkflowPopupID is the identifier for the workflow popup dialog.
const WorkflowPopupID = "workflow_popup"

const (
	defaultWorkflowPopupMaxWidth  = 80
	defaultWorkflowPopupMaxHeight = 24
)

// WorkflowAgentInfo represents live progress state for one sub-agent.
type WorkflowAgentInfo struct {
	Index   int
	Label   string
	Status  string // "queued", "running", "done", "error"
	Error   string
	LastLog string
	Logs    []string
}

// WorkflowPopup is a dialog overlay that displays live progress of a running workflow.
type WorkflowPopup struct {
	com         *common.Common
	toolCallID  string
	description string
	startTime   time.Time
	finished    bool
	dismissed   bool

	running   int
	completed int
	total     int
	lastSeq   int64

	agents     []*WorkflowAgentInfo
	agentMap   map[int]*WorkflowAgentInfo
	globalLogs []string

	selectedIndex int
	logScroll     int

	help   help.Model
	keyMap struct {
		Close    key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		PrevLog  key.Binding
		NextLog  key.Binding
	}
}

var _ Dialog = (*WorkflowPopup)(nil)

// NewWorkflowPopup creates a new [WorkflowPopup].
func NewWorkflowPopup(com *common.Common, toolCallID string) *WorkflowPopup {
	w := &WorkflowPopup{
		com:           com,
		toolCallID:    toolCallID,
		startTime:     time.Now(),
		agentMap:      make(map[int]*WorkflowAgentInfo),
		selectedIndex: 0,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	w.help = help

	w.keyMap.Close = key.NewBinding(
		key.WithKeys("esc", "q"),
		key.WithHelp("esc/q", "close"),
	)
	w.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "j", "ctrl+n"),
		key.WithHelp("↓/j", "next agent"),
	)
	w.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "k", "ctrl+p"),
		key.WithHelp("↑/k", "prev agent"),
	)
	w.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down", "j", "k"),
		key.WithHelp("↑/↓", "select agent"),
	)
	w.keyMap.PrevLog = key.NewBinding(
		key.WithKeys("pgup", "u"),
		key.WithHelp("u/pgup", "log up"),
	)
	w.keyMap.NextLog = key.NewBinding(
		key.WithKeys("pgdown", "d"),
		key.WithHelp("d/pgdn", "log down"),
	)

	return w
}

// ID implements [Dialog].
func (w *WorkflowPopup) ID() string {
	return WorkflowPopupID
}

// ToolCallID returns the tool call ID this popup is tracking.
func (w *WorkflowPopup) ToolCallID() string {
	return w.toolCallID
}

// SetDescription sets the workflow description.
func (w *WorkflowPopup) SetDescription(desc string) {
	w.description = desc
}

// IsFinished returns whether the workflow has completed.
func (w *WorkflowPopup) IsFinished() bool {
	return w.finished
}

// SetFinished sets the finished state.
func (w *WorkflowPopup) SetFinished(f bool) {
	w.finished = f
}

// IsDismissed returns whether the user manually dismissed the popup.
func (w *WorkflowPopup) IsDismissed() bool {
	return w.dismissed
}

// SetDismissed sets the dismissed state.
func (w *WorkflowPopup) SetDismissed(d bool) {
	w.dismissed = d
}

// HandleProgress processes a WorkflowProgress notification.
func (w *WorkflowPopup) HandleProgress(wp *notify.WorkflowProgress) {
	if wp == nil {
		return
	}

	// Seq restarts at 1 for each workflow run. Treat that as the start of a
	// new stream so a retained popup (same tool call ID) cannot swallow it.
	if wp.Seq == 1 {
		w.lastSeq = 0
	}
	if wp.Seq != 0 && wp.Seq <= w.lastSeq {
		return
	}
	w.lastSeq = wp.Seq

	w.running = wp.Running
	w.completed = wp.Completed
	w.total = wp.Total

	if wp.Total > len(w.agents) {
		w.ensureTotalAgents(wp.Total)
	}

	if wp.Kind == "agent_start" && wp.Index >= 0 {
		agent := w.getOrOrCreateAgent(wp.Index, wp.Label)
		agent.Status = "running"
		if wp.Label != "" {
			agent.Label = wp.Label
		}
	} else if wp.Kind == "agent_done" && wp.Index >= 0 {
		agent := w.getOrOrCreateAgent(wp.Index, wp.Label)
		agent.Status = "done"
		if wp.Label != "" {
			agent.Label = wp.Label
		}
	} else if wp.Kind == "agent_error" && wp.Index >= 0 {
		agent := w.getOrOrCreateAgent(wp.Index, wp.Label)
		agent.Status = "error"
		if wp.Label != "" {
			agent.Label = wp.Label
		}
		if wp.Message != "" {
			agent.Error = wp.Message
			agent.LastLog = wp.Message
			agent.Logs = append(agent.Logs, wp.Message)
		}
	} else if wp.Kind == "log" {
		if wp.Index >= 0 {
			agent := w.getOrOrCreateAgent(wp.Index, wp.Label)
			if wp.Message != "" {
				agent.LastLog = wp.Message
				agent.Logs = append(agent.Logs, wp.Message)
			}
		} else if wp.Message != "" {
			w.globalLogs = append(w.globalLogs, wp.Message)
			for _, a := range w.agents {
				if a != nil && a.Status == "running" {
					a.LastLog = wp.Message
				}
			}
		}
	}

	if w.selectedIndex < 0 && len(w.agents) > 0 {
		w.selectedIndex = 0
	}
}

func (w *WorkflowPopup) ensureTotalAgents(total int) {
	for i := len(w.agents); i < total; i++ {
		agent := &WorkflowAgentInfo{
			Index:  i,
			Label:  fmt.Sprintf("Agent %d", i+1),
			Status: "queued",
		}
		w.agents = append(w.agents, agent)
		w.agentMap[i] = agent
	}
}

func (w *WorkflowPopup) getOrOrCreateAgent(idx int, label string) *WorkflowAgentInfo {
	if agent, ok := w.agentMap[idx]; ok {
		return agent
	}
	defaultLabel := label
	if defaultLabel == "" {
		defaultLabel = fmt.Sprintf("Agent %d", idx+1)
	}
	agent := &WorkflowAgentInfo{
		Index:  idx,
		Label:  defaultLabel,
		Status: "queued",
	}
	for len(w.agents) <= idx {
		w.agents = append(w.agents, nil)
	}
	w.agents[idx] = agent
	w.agentMap[idx] = agent
	return agent
}

// HandleMsg implements [Dialog].
func (w *WorkflowPopup) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, w.keyMap.Close):
			w.dismissed = true
			return ActionClose{}
		case key.Matches(msg, w.keyMap.Previous):
			if w.selectedIndex > 0 {
				w.selectedIndex--
				w.logScroll = 0
			}
		case key.Matches(msg, w.keyMap.Next):
			if w.selectedIndex < len(w.agents)-1 {
				w.selectedIndex++
				w.logScroll = 0
			}
		case key.Matches(msg, w.keyMap.PrevLog):
			if w.logScroll > 0 {
				w.logScroll--
			}
		case key.Matches(msg, w.keyMap.NextLog):
			w.logScroll++
		}
	}
	return nil
}

// ShortHelp implements [help.KeyMap].
func (w *WorkflowPopup) ShortHelp() []key.Binding {
	return []key.Binding{
		w.keyMap.UpDown,
		w.keyMap.PrevLog,
		w.keyMap.NextLog,
		w.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (w *WorkflowPopup) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			w.keyMap.UpDown,
			w.keyMap.PrevLog,
			w.keyMap.NextLog,
			w.keyMap.Close,
		},
	}
}

// Draw implements [Dialog].
func (w *WorkflowPopup) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := w.com.Styles
	width := max(0, min(defaultWorkflowPopupMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultWorkflowPopupMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	_ = height
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Title = "Workflow Execution"
	if w.description != "" {
		rc.Title = "Workflow: " + ansi.Truncate(w.description, innerWidth-25, "…")
	}

	var validAgents []*WorkflowAgentInfo
	for _, a := range w.agents {
		if a != nil {
			validAgents = append(validAgents, a)
		}
	}

	if w.total == 0 && len(validAgents) == 0 {
		rc.Title = "Workflow Engine (Idle)"
		rc.TitleInfo = "0/0 done"
		rc.AddPart(t.Dialog.SecondaryText.Render("No active workflow is currently running."))
		rc.AddPart(t.Dialog.SecondaryText.Render("Workflows orchestrate multiple sub-agents in parallel or sequence."))
		rc.Help = renderDialogHelp(t, &w.help, w, innerWidth)
		view := rc.Render()
		DrawCenter(scr, area, view)
		return nil
	}

	elapsed := time.Since(w.startTime).Truncate(time.Second)
	mins := int(elapsed.Minutes())
	secs := int(elapsed.Seconds()) % 60
	timeStr := fmt.Sprintf("%02d:%02d", mins, secs)

	rc.TitleInfo = fmt.Sprintf("[%s] %d/%d done", timeStr, w.completed, w.total)

	// Progress bar line
	barWidth := max(10, innerWidth-28)
	pct := 0.0
	if w.total > 0 {
		pct = float64(w.completed) / float64(w.total)
	}
	filledLen := int(pct * float64(barWidth))
	if filledLen > barWidth {
		filledLen = barWidth
	}
	barStr := strings.Repeat("█", filledLen) + strings.Repeat("░", barWidth-filledLen)

	progressLine := fmt.Sprintf("%s  %d/%d done", barStr, w.completed, w.total)
	if w.running > 0 {
		progressLine += fmt.Sprintf(" (%d running)", w.running)
	}
	rc.AddPart(t.Dialog.SecondaryText.Render(progressLine))

	// Sub-agents section
	rc.AddPart(t.Dialog.PrimaryText.Bold(true).Render("Sub-Agents:"))

	maxAgentListLines := 5

	if len(validAgents) == 0 {
		rc.AddPart(t.Dialog.SecondaryText.Render("  (waiting for agents to start...)"))
	} else {
		if w.selectedIndex >= len(validAgents) {
			w.selectedIndex = max(0, len(validAgents)-1)
		}
		for i, agent := range validAgents {
			if i >= maxAgentListLines {
				more := fmt.Sprintf("  … and %d more sub-agents", len(validAgents)-maxAgentListLines)
				rc.AddPart(t.Dialog.SecondaryText.Render(more))
				break
			}
			icon := "·"
			iconStyle := t.Dialog.SecondaryText
			switch agent.Status {
			case "running":
				icon = "⟳"
				iconStyle = t.Tool.TodoInProgressIcon
			case "done":
				icon = "✓"
				iconStyle = t.Tool.TodoCompletedIcon
			case "error":
				icon = "✗"
				iconStyle = t.Tool.IconError
			case "queued":
				icon = "·"
				iconStyle = t.Tool.TodoPendingIcon
			}

			prefix := "  "
			if i == w.selectedIndex {
				prefix = "▶ "
			}

			labelStr := agent.Label
			if labelStr == "" {
				labelStr = fmt.Sprintf("Agent %d", agent.Index+1)
			}

			rowText := fmt.Sprintf("%s%s %d. %s", prefix, iconStyle.Render(icon), agent.Index+1, labelStr)
			if agent.LastLog != "" {
				logAvailWidth := max(10, innerWidth-lipgloss.Width(rowText)-5)
				rowText += t.Dialog.SecondaryText.Render(" — " + ansi.Truncate(agent.LastLog, logAvailWidth, "…"))
			}

			if i == w.selectedIndex {
				rc.AddPart(t.Dialog.SelectedItem.Width(innerWidth).Render(rowText))
			} else {
				rc.AddPart(t.Dialog.NormalItem.Width(innerWidth).Render(rowText))
			}
		}
	}

	// Logs section
	var selAgent *WorkflowAgentInfo
	if w.selectedIndex >= 0 && w.selectedIndex < len(validAgents) {
		selAgent = validAgents[w.selectedIndex]
	}

	logTitle := "Live Logs"
	if selAgent != nil {
		logTitle = fmt.Sprintf("Logs for %s", selAgent.Label)
	}
	rc.AddPart(t.Dialog.PrimaryText.Bold(true).Render(logTitle + ":"))

	var logsToShow []string
	if selAgent != nil && len(selAgent.Logs) > 0 {
		logsToShow = selAgent.Logs
	} else if selAgent != nil && selAgent.Error != "" {
		logsToShow = []string{"Error: " + selAgent.Error}
	} else if len(w.globalLogs) > 0 {
		logsToShow = w.globalLogs
	}

	if len(logsToShow) == 0 {
		rc.AddPart(t.Dialog.SecondaryText.Render("  (no logs recorded yet)"))
	} else {
		maxLogLines := 4
		start := max(0, len(logsToShow)-maxLogLines-w.logScroll)
		end := min(len(logsToShow), start+maxLogLines)
		if start > end {
			start = 0
			end = min(len(logsToShow), maxLogLines)
		}
		for j := start; j < end; j++ {
			rc.AddPart(t.Dialog.SecondaryText.Render("  " + ansi.Truncate(logsToShow[j], innerWidth-4, "…")))
		}
	}

	rc.Help = renderDialogHelp(t, &w.help, w, innerWidth)
	view := rc.Render()

	DrawCenter(scr, area, view)
	return nil
}
